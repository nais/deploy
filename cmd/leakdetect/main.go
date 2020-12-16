package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var occ = make(map[string]int)

type deploy struct {
	id   string
	seen int
}

var (
	interval  = flag.Duration("interval", 1*time.Minute, "poll interval")
	threshold = flag.Int("threshold", 30, "interesting threshold")
)

func main() {
	flag.Parse()

	ticker := time.NewTicker(*interval)

	for range ticker.C {
		data, err := http.Get("https://deploy.nais.io/api/v1/queue")
		if err != nil {
			log.Error(err)
			continue
		}

		dest := make(map[string]interface{})
		dec := json.NewDecoder(data.Body)
		err = dec.Decode(&dest)
		if err != nil {
			log.Error(err)
			continue
		}

		for k := range dest {
			occ[k]++
		}

		deploys := sorted()
		show(deploys)
	}
}

func sorted() []deploy {
	slice := make([]deploy, 0, len(occ))
	for k, v := range occ {
		slice = append(slice, deploy{
			id:   k,
			seen: v,
		})
	}
	sort.Slice(slice, func(i, j int) bool {
		return slice[i].seen < slice[j].seen
	})
	return slice
}

func show(deploys []deploy) {
	print("\033[H\033[2J")
	for _, v := range deploys {
		if v.seen > *threshold {
			log.Infof("%3d %s", v.seen, v.id)
		}
	}
}
