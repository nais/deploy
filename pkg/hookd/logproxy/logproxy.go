package logproxy

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
)

var uuidRegex = regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$")

func MakeURL(baseURL, deliveryID string, timestamp time.Time) string {
	return fmt.Sprintf("%s/logs?delivery_id=%s&ts=%d&v=1", baseURL, deliveryID, timestamp.Unix())
}

func MakeHandler() http.HandlerFunc {
	formatterFunc := formatKibana
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
		url, err := formatterFunc(deliveryID, utctime, version)
		if err != nil {
			badrequest(err)
			return
		}

		http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	}
}
