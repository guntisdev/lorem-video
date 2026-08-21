package service

import "fmt"

func SliceLoopPeriod(clusters []WebMCluster, periodMs, clusterMs uint64) ([]WebMCluster, error) {
	if len(clusters) == 0 {
		return nil, fmt.Errorf("no clusters")
	}

	if end := clusters[len(clusters)-1].TimecodeMs + clusterMs; end < 3*periodMs {
		return nil, fmt.Errorf("input covers %dms, need %dms", end, 3*periodMs)
	}

	var kept []WebMCluster
	for _, c := range clusters {
		if c.TimecodeMs >= periodMs && c.TimecodeMs < 2*periodMs {
			kept = append(kept, c)
		}
	}

	want := int(periodMs / clusterMs)
	if len(kept) != want {
		return nil, fmt.Errorf("sliced %d clusters, want %d", len(kept), want)
	}

	base := kept[0].TimecodeMs
	out := make([]WebMCluster, len(kept))

	for i, c := range kept {
		rel := c.TimecodeMs - base
		if rel != uint64(i)*clusterMs {
			return nil, fmt.Errorf("cluster %d at %dms, want %dms", i, rel, uint64(i)*clusterMs)
		}
		if err := PatchClusterTimecode(c.Data, rel); err != nil {
			return nil, err
		}
		c.TimecodeMs = rel
		out[i] = c
	}

	return out, nil
}
