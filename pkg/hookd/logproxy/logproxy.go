package logproxy

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

type Config struct {
	Projects map[string]string
}

var uuidRegex = regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$")

func MakeURL(baseURL, deliveryID string, timestamp time.Time, cluster string) string {
	return fmt.Sprintf("%s/logs?delivery_id=%s&ts=%d&v=1&cluster=%s", baseURL, deliveryID, timestamp.Unix(), cluster)
}

func MakeHandler(cfg Config) http.HandlerFunc {
	var formatterFunc func(deliveryID string, ts time.Time, version int, cluster string) (string, error)
	if len(cfg.Projects) > 0 {
		formatter := gcpFormatter{Projects: cfg.Projects}
		formatterFunc = formatter.format
		log.Infof("Configured logproxy to target Google Logs Explorer")
	} else {
		formatterFunc = formatKibana
		log.Infof("Configured logproxy to target Kibana")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		badrequest := func(err error) {
			w.WriteHeader(http.StatusBadRequest)
			log.Error(err)
			_, err = w.Write([]byte(err.Error() + "\n"))
			if err != nil {
				log.Errorf("unable to answer http request: %s", err)
			}
		}
		deliveryID := r.URL.Query().Get("delivery_id")
		timestamp := r.URL.Query().Get("ts")
		sversion := r.URL.Query().Get("v")
		version, _ := strconv.Atoi(sversion)
		cluster := r.URL.Query().Get("cluster")

		if !uuidRegex.MatchString(deliveryID) {
			badrequest(fmt.Errorf("delivery_id '%s' is not a well-formed UUID", deliveryID))
			return
		}

		unixtime, err := strconv.Atoi(timestamp)
		if err != nil {
			badrequest(fmt.Errorf("ts '%s' is not a well-formed unix timestamp: %s", timestamp, err))
			return
		}

		utctime := time.Unix(int64(unixtime), 0).UTC()
		url, err := formatterFunc(deliveryID, utctime, version, cluster)
		if err != nil {
			badrequest(err)
			return
		}

		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}
