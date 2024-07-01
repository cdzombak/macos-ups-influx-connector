package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/cdzombak/heartbeat"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

var version = "<dev>"

type parsedUPSLine struct {
	model                string
	id                   string
	acAttached           bool
	charging             bool
	batteryChargePercent int
}

func readPmSet() ([]parsedUPSLine, error) {
	pmsetCmd := exec.Command("pmset", "-g", "batt")
	pmsetOut, err := pmsetCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pmset read failed: %w", err)
	}
	// Now drawing from 'AC Power'
	// -CP1500PFCLCD (id=17825794)	100%; AC attached; not charging present: true
	lines := strings.Split(string(pmsetOut), "\n")
	var retv []parsedUPSLine
	for i, l := range lines {
		if i == 0 {
			continue
		}
		if len(l) == 0 {
			continue
		}
		lParsed := parsedUPSLine{}
		l = strings.TrimSpace(l)
		lParts := strings.Split(l, "\t")
		if len(lParts) != 2 {
			log.Printf("failed to parse pmset line (err 1): '%s'", l)
			continue
		}
		nameAndIDLineParts := strings.Split(lParts[0], " ")
		if len(nameAndIDLineParts) != 2 {
			log.Printf("failed to parse pmset line (err 2): '%s'", l)
			continue
		}
		lParsed.model = nameAndIDLineParts[0][1:]
		lParsed.id = nameAndIDLineParts[1][4 : len(nameAndIDLineParts[1])-1]
		if strings.Contains(lParts[1], "AC attached") {
			lParsed.acAttached = true
		}
		if strings.Contains(lParts[1], "not charging") {
			lParsed.charging = false
		} else if strings.Contains(lParts[1], "charging") {
			lParsed.charging = true
		}
		if !strings.Contains(lParts[1], "present: true") {
			// only include present UPSs/batteries
			continue
		}
		pctSplitParts := strings.Split(lParts[1], "%")
		if len(pctSplitParts) != 2 {
			log.Printf("failed to parse pmset line (err 3): '%s'", l)
			continue
		}
		pctStr := strings.TrimSpace(pctSplitParts[0])
		pct, err := strconv.Atoi(pctStr)
		if err != nil {
			log.Printf("failed to parse string '%s' into int: '%s'", pctStr, err)
			continue
		}
		lParsed.batteryChargePercent = pct
		retv = append(retv, lParsed)
	}
	return retv, nil
}

func main() {
	var influxServer = flag.String("influx-server", "", "InfluxDB server, including protocol and port, eg. 'http://192.168.1.1:8086'. Required.")
	var influxUser = flag.String("influx-username", "", "InfluxDB username.")
	var influxPass = flag.String("influx-password", "", "InfluxDB password.")
	var influxBucket = flag.String("influx-bucket", "", "InfluxDB bucket. Supply a string in the form 'database/retention-policy'. For the default retention policy, pass just a database name (without the slash character). Required.")
	var measurementName = flag.String("measurement-name", "ups_stats", "InfluxDB measurement name.")
	var upsNameTag = flag.String("ups-nametag", "", "Value for the ups_name tag in InfluxDB. Required.")
	var pollInterval = flag.Int("poll-interval", 30, "Polling interval, in seconds.")
	var influxTimeoutS = flag.Int("influx-timeout", 3, "Timeout for writing to InfluxDB, in seconds.")
	var heartbeatURL = flag.String("heartbeat-url", "", "URL to GET every 60s, if and only if the program has successfully sent UPS statistics to Influx in the past 120s.")
	var printVersion = flag.Bool("version", false, "Print version and exit.")
	flag.Parse()

	if *printVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if *influxServer == "" || *influxBucket == "" {
		fmt.Println("-influx-bucket and -influx-server must be supplied.")
		os.Exit(1)
	}
	if *upsNameTag == "" {
		fmt.Println("-ups-nametag must be supplied.")
		os.Exit(1)
	}

	var hb heartbeat.Heartbeat
	var err error
	if *heartbeatURL != "" {
		hb, err = heartbeat.NewHeartbeat(&heartbeat.Config{
			HeartbeatInterval: 60 * time.Second,
			LivenessThreshold: 120 * time.Second,
			HeartbeatURL:      *heartbeatURL,
			OnError: func(err error) {
				log.Printf("heartbeat error: %s\n", err)
			},
		})
		if err != nil {
			log.Fatalf("failed to create heartbeat client: %v", err)
		}
	}

	influxTimeout := time.Duration(*influxTimeoutS) * time.Second
	authString := ""
	if *influxUser != "" || *influxPass != "" {
		authString = fmt.Sprintf("%s:%s", *influxUser, *influxPass)
	}
	influxClient := influxdb2.NewClient(*influxServer, authString)
	ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
	defer cancel()
	health, err := influxClient.Health(ctx)
	if err != nil {
		log.Fatalf("failed to check InfluxDB health: %v", err)
	}
	if health.Status != "pass" {
		log.Fatalf("InfluxDB did not pass health check: status %s; message '%s'", health.Status, *health.Message)
	}
	influxWriteAPI := influxClient.WriteAPIBlocking("", *influxBucket)

	doUpdate := func() {
		atTime := time.Now()

		upsList, err := readPmSet()
		if err != nil {
			log.Println(err.Error())
			return
		}

		var points []*write.Point
		for _, ups := range upsList {
			ntValue := *upsNameTag
			if len(upsList) > 1 {
				ntValue = fmt.Sprintf("%s|%s", ntValue, ups.id)
			}

			// tags:
			// ups/model
			// ups/name append ID iff >1 UPS
			// ups/id

			// data:
			// battery charge (percent) (int)
			// AC attached (bool)

			points = append(points, influxdb2.NewPoint(
				*measurementName,
				map[string]string{ // tags
					"ups_name":  ntValue,
					"ups_model": ups.model,
					"ups_id":    ups.id,
				},
				map[string]interface{}{ // fields
					"battery_charge_percent": ups.batteryChargePercent,
					"ac_attached":            ups.acAttached,
				},
				atTime,
			))
		}

		if err := retry.Do(
			func() error {
				ctx, cancel := context.WithTimeout(context.Background(), influxTimeout)
				defer cancel()
				return influxWriteAPI.WritePoint(ctx, points...)
			},
			retry.Attempts(2),
		); err != nil {
			log.Printf("failed to write to influx: %s", err.Error())
		} else if hb != nil {
			hb.Alive(atTime)
		}
	}

	if hb != nil {
		hb.Start()
	}
	doUpdate()
	for range time.Tick(time.Duration(*pollInterval) * time.Second) {
		doUpdate()
	}
}
