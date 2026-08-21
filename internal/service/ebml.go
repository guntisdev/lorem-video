// EBML is the container format under webm and mkv. It works like binary XML:
// every element is an id vint, a size vint, then the payload.
//
// Vint length comes from the leading zeros of the first byte, so each field
// describes its own width:
//
//	0x80-0xFF  1 byte
//	0x40-0x7F  2 bytes
//	0x20-0x3F  3 bytes
//	0x01       8 bytes
//
// In a size vint the remaining bits hold the value. All bits set means unknown
// size, which ffmpeg writes for Segment with -live 1.
//
//	1a45dfa3 9f                 EBML header, 31 bytes
//	  4282 84 "webm"              DocType
//	18538067 01ffffffffffffff   Segment, unknown size
//	  1654ae6b 40b3               Tracks, 179 bytes
//	  1f43b675 215294             Cluster, 86676 bytes
//	    e7 82 03d5                  Timecode, 981ms
//	    a3 40b6 ...                 SimpleBlock, int16 offset from Timecode
//
// Elements are self delimiting, so we only read Segment, Cluster and Timecode
// and copy the vp8 and opus payloads without parsing them.

package service

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

const (
	ebmlSegment  = 0x18538067
	ebmlCluster  = 0x1F43B675
	ebmlTimecode = 0xE7
)

// Timecode offset in a normalized cluster: 4 byte id + 8 byte size + 2 byte header.
const WSTimecodeOffset = 14

type WebMCluster struct {
	TimecodeMs uint64
	Data       []byte
}

type WebMSplit struct {
	Init     []byte // bytes before the first cluster
	Clusters []WebMCluster
}

// SplitWebM returns the init segment and the normalized clusters.
func SplitWebM(d []byte) (*WebMSplit, error) {
	segStart, segEnd, err := findSegment(d)
	if err != nil {
		return nil, err
	}

	split := &WebMSplit{}

	for p := segStart; p < segEnd; {
		id, idLen, _, err := readVint(d, p, true)
		if err != nil {
			return nil, err
		}
		size, sizeLen, unknown, err := readVint(d, p+idLen, false)
		if err != nil {
			return nil, err
		}
		if unknown {
			return nil, fmt.Errorf("element %#x has unknown size", id)
		}

		body := p + idLen + sizeLen
		end := body + int(size)
		if end > segEnd {
			return nil, fmt.Errorf("element %#x overruns segment", id)
		}

		if id == ebmlCluster {
			if split.Init == nil {
				split.Init = d[:p:p]
			}
			cluster, err := normalizeCluster(d[body:end])
			if err != nil {
				return nil, err
			}
			split.Clusters = append(split.Clusters, cluster)
		}

		p = end
	}

	if len(split.Clusters) == 0 {
		return nil, fmt.Errorf("no clusters found")
	}

	return split, nil
}

// PatchClusterTimecode sets the cluster's absolute timecode.
func PatchClusterTimecode(chunk []byte, absMs uint64) error {
	if len(chunk) < WSTimecodeOffset+8 {
		return fmt.Errorf("chunk too short: %d bytes", len(chunk))
	}
	binary.BigEndian.PutUint64(chunk[WSTimecodeOffset:WSTimecodeOffset+8], absMs)
	return nil
}

func findSegment(d []byte) (start, end int, err error) {
	for p := 0; p < len(d); {
		id, idLen, _, err := readVint(d, p, true)
		if err != nil {
			return 0, 0, err
		}
		size, sizeLen, unknown, err := readVint(d, p+idLen, false)
		if err != nil {
			return 0, 0, err
		}

		body := p + idLen + sizeLen
		if id == ebmlSegment {
			if unknown {
				return body, len(d), nil
			}
			return body, min(body+int(size), len(d)), nil
		}

		if unknown {
			return 0, 0, fmt.Errorf("element %#x has unknown size before segment", id)
		}
		p = body + int(size)
	}

	return 0, 0, fmt.Errorf("no segment found")
}

// normalizeCluster widens the header to a fixed size so the timecode lands at
// WSTimecodeOffset and can be patched in place.
func normalizeCluster(payload []byte) (WebMCluster, error) {
	id, idLen, _, err := readVint(payload, 0, true)
	if err != nil {
		return WebMCluster{}, err
	}
	if id != ebmlTimecode {
		return WebMCluster{}, fmt.Errorf("cluster starts with %#x, want timecode", id)
	}

	size, sizeLen, _, err := readVint(payload, idLen, false)
	if err != nil {
		return WebMCluster{}, err
	}

	tcStart := idLen + sizeLen
	tcEnd := tcStart + int(size)
	if size > 8 || tcEnd > len(payload) {
		return WebMCluster{}, fmt.Errorf("bad timecode size %d", size)
	}

	var timecode uint64
	for _, b := range payload[tcStart:tcEnd] {
		timecode = timecode<<8 | uint64(b)
	}

	body := payload[tcEnd:]

	// 8 byte vint: marker plus 7 bytes of length
	var sizeField [8]byte
	binary.BigEndian.PutUint64(sizeField[:], uint64(10+len(body)))
	sizeField[0] = 0x01

	data := make([]byte, 0, WSTimecodeOffset+8+len(body))
	data = append(data, 0x1F, 0x43, 0xB6, 0x75)
	data = append(data, sizeField[:]...)
	data = append(data, ebmlTimecode, 0x88)
	data = binary.BigEndian.AppendUint64(data, timecode)
	data = append(data, body...)

	return WebMCluster{TimecodeMs: timecode, Data: data}, nil
}

// readVint reads a vint. keepMarker keeps the length bits, as used by element ids.
func readVint(d []byte, p int, keepMarker bool) (val uint64, n int, unknown bool, err error) {
	if p >= len(d) {
		return 0, 0, false, fmt.Errorf("vint past end at %d", p)
	}

	first := d[p]
	if first == 0 {
		return 0, 0, false, fmt.Errorf("invalid vint at %d", p)
	}

	n = 8 - bits.Len8(first) + 1
	if p+n > len(d) {
		return 0, 0, false, fmt.Errorf("truncated vint at %d", p)
	}

	for _, b := range d[p : p+n] {
		val = val<<8 | uint64(b)
	}

	if !keepMarker {
		mask := uint64(1)<<(7*n) - 1
		unknown = val == mask
		val &= mask
	}

	return val, n, unknown, nil
}
