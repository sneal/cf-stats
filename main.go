package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry-community/go-cfclient/v3/client"
	"github.com/cloudfoundry-community/go-cfclient/v3/config"
	"github.com/cloudfoundry-community/go-cfclient/v3/resource"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
)

var cf *client.Client

type HostToAppsDetail struct {
	Host string
	Apps []string
}

type VcapApplication struct {
	CFAPI string `json:"cf_api"`
}

// IndexHandler is the default root handler
func IndexHandler(w http.ResponseWriter, r *http.Request) {
	results, err := listApps(context.Background())
	if err != nil {
		log.Printf(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)

	_, _ = fmt.Fprintf(w, "<h1>CF Application Placements</h1>")
	for _, d := range results {
		_, _ = fmt.Fprintf(w, "<h2>Host: %s</h2>", d.Host)
		_, _ = fmt.Fprint(w, "<ul>")
		for _, a := range d.Apps {
			_, _ = fmt.Fprintf(w, "<li>%s</li>", a)
		}
		_, _ = fmt.Fprint(w, "</ul>")
	}
}

func listApps(ctx context.Context) ([]*HostToAppsDetail, error) {
	unsortedResults := make(map[string]*HostToAppsDetail)
	opts := client.NewAppListOptions()
	for {
		apps, pager, err := cf.Applications.List(ctx, opts)
		if err != nil {
			return nil, err
		}
		for _, app := range apps {
			err := listProcessStats(ctx, app, unsortedResults)
			if err != nil {
				return nil, err
			}
		}
		if !pager.HasNextPage() {
			break
		}
		pager.NextPage(opts)
	}

	// convert results into an array of results, then sort by host IP
	var results []*HostToAppsDetail
	for _, v := range unsortedResults {
		sort.Strings(v.Apps)
		results = append(results, v)
	}

	sort.Slice(results, func(i, j int) bool {
		return bytes.Compare(net.ParseIP(results[i].Host), net.ParseIP(results[j].Host)) < 0
	})

	return results, nil
}

func listProcessStats(ctx context.Context, app *resource.App, results map[string]*HostToAppsDetail) error {
	stats, err := cf.Processes.GetStatsForApp(ctx, app.GUID, "web")
	if err != nil {
		return err
	}

	for _, s := range stats.Stats {
		host := s.Host
		d, ok := results[host]
		if !ok {
			d = &HostToAppsDetail{
				Host: host,
				Apps: []string{},
			}
			results[host] = d
		}
		d.Apps = append(d.Apps, app.Name)
	}

	return nil
}

func main() {
	var cfEndpoint string
	v := os.Getenv("VCAP_APPLICATION")
	if v != "" {
		var a VcapApplication
		err := json.NewDecoder(strings.NewReader(v)).Decode(&a)
		if err != nil {
			log.Fatalf("failed to read VCAP_APPLICATION: %s", err.Error())
		}
		cfEndpoint = a.CFAPI
	}
	var user string
	if user = os.Getenv("CF_USER"); len(user) == 0 {
		log.Fatal("failed to read CF_USER")
	}
	var password string
	if password = os.Getenv("CF_PASSWORD"); len(password) == 0 {
		log.Fatal("failed to read CF_PASSWORD")
	}
	var port string
	if port = os.Getenv("PORT"); len(port) == 0 {
		port = "8080"
	}

	cfg, err := config.New(cfEndpoint, config.UserPassword(user, password), config.SkipTLSValidation())
	if err != nil {
		log.Fatalf("failed to create CF config: %s", err.Error())
	}
	cf, err = client.New(cfg)
	if err != nil {
		log.Fatalf("failed to create CF client: %s", err.Error())
	}

	http.HandleFunc("/", IndexHandler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
