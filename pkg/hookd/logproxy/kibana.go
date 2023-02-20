package logproxy

import (
	"fmt"
	"time"

	"gopkg.in/sakura-internet/go-rison.v3"
)

const (
	defaultIndex  = "96e648c0-980a-11e9-830a-e17bbd64b4db"
	kibanaFormat  = "https://logs.adeo.no/app/kibana#/discover?_a=%s&_g=%s"
	searchQueryV0 = "+x_delivery_id:\"%s\" -level:\"Trace\""
	searchQueryV1 = "+x_correlation_id:\"%s\" -level:\"Trace\" -level:\"Debug\""
)

type query struct {
	Language string `json:"language"`
	Query    string `json:"query"`
}

type appState struct {
	Index string `json:"index"`
	Query query  `json:"query"`
}

type timeRange struct {
	From string `json:"from"`
	Mode string `json:"mode"`
	To   string `json:"to"`
}

type globalState struct {
	Time timeRange `json:"time"`
}

func oneDay(ts time.Time) timeRange {
	od := time.Hour * 24
	start := ts.Truncate(od)
	end := start.Add(od)

	startStr, _ := start.MarshalText()
	endStr, _ := end.MarshalText()

	return timeRange{
		From: string(startStr),
		Mode: "absolute",
		To:   string(endStr),
	}
}

func formatKibana(deliveryID string, ts time.Time, version int, _ string) (string, error) {
	var q string

	switch version {
	case 0:
		q = searchQueryV0
	case 1:
		q = searchQueryV1
	default:
		return "", fmt.Errorf("version '%s' is not supported", deliveryID)
	}

	as := appState{
		Index: defaultIndex,
		Query: query{
			Language: "lucene",
			Query:    fmt.Sprintf(q, deliveryID),
		},
	}

	gs := globalState{
		Time: oneDay(ts),
	}

	b, _ := rison.Encode(as, rison.Rison)
	c, _ := rison.Encode(gs, rison.Rison)

	return fmt.Sprintf(kibanaFormat, string(b), string(c)), nil
}
