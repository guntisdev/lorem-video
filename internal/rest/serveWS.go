package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"lorem.video/internal/config"
)

const wsVideoCodec = "vp8"
const wsAudioCodec = "opus"

type wsManifest struct {
	Name    string           `json:"name"`
	Streams []wsManifestItem `json:"streams"`
}

type wsManifestItem struct {
	StreamName string `json:"streamName"`
	URL        string `json:"url"`
	StreamID   string `json:"streamId"`
	Bitrate    string `json:"bitrate"`
	VC         string `json:"vc"`
}

func (rest *Rest) ServeWSManifest(w http.ResponseWriter, r *http.Request) {
	videoName := r.PathValue("videoName")
	if videoName == "" {
		videoName = config.DefaultVideoSpec.Name
	}

	if _, err := os.Stat(filepath.Join(config.AppPaths.WSStream, videoName)); os.IsNotExist(err) {
		http.Error(w, "Stream not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")

	if err := json.NewEncoder(w).Encode(buildWSManifest(videoName)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func buildWSManifest(videoName string) wsManifest {
	baseURL := config.GetWSBaseURL()
	streams := make([]wsManifestItem, 0, len(config.WSRenditions))

	for _, res := range config.WSRenditions {
		bitrate, ok := config.WSBitrates[res]
		if !ok {
			continue
		}

		streamName := videoName + "_" + config.WSStreamSuffix[res]

		streams = append(streams, wsManifestItem{
			StreamName: streamName,
			URL: fmt.Sprintf("%s/ws/%s/websocketstream2?vc=%s&ac=%s",
				baseURL, streamName, wsVideoCodec, wsAudioCodec),
			StreamID: config.ResolutionsName[res],
			Bitrate:  strconv.Itoa(bitrate.Video + bitrate.Audio),
			VC:       wsVideoCodec,
		})
	}

	return wsManifest{Name: videoName + "_auto", Streams: streams}
}
