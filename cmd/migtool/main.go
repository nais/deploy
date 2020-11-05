// migtool is a migration tool for converting team repository data from dumped S3 format
// into a postgresql database.
//
// prerequisites:
//
// $ mkdir /tmp/migrate
// $ cd /tmp/migrate
// $ s3cmd get -r s3://deployments.nais.io/navikt
//
// run migtool:
// $ migtool -source /tmp/migrate/navikt -url postgres://...

package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"

	"github.com/navikt/deployment/pkg/hookd/database"
)

func main() {
	url := flag.String("url", "postgres://postgres:root@localhost/hookd", "postgresql url")
	source := flag.String("source", ".", "source of data")

	flag.Parse()

	db, err := database.New(*url, nil)
	if err != nil {
		log.Fatal(err)
	}

	files, err := ioutil.ReadDir(*source)
	if err != nil {
		log.Fatal(err)
	}

	for _, fn := range files {
		if fn.IsDir() {
			continue
		}
		path := filepath.Join(*source, fn.Name())
		file, err := os.Open(path)
		if err != nil {
			log.Fatal(err)
		}

		repository := "navikt/" + fn.Name()
		teams := make([]string, 0)
		decoder := json.NewDecoder(file)
		err = decoder.Decode(&teams)
		if err != nil {
			log.Fatal(err)
		}
		err = file.Close()
		if err != nil {
			log.Fatal(err)
		}

		err = db.WriteRepositoryTeams(context.Background(), repository, teams)
		if err != nil {
			log.Fatal(err)
		}

		log.Infof("wrote %s -> %v", repository, teams)
	}
}
