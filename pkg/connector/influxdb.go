// +build influxdb

package connector

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/facette/facette/pkg/catalog"
	"github.com/facette/facette/pkg/config"
	"github.com/facette/facette/pkg/logger"
	"github.com/facette/facette/pkg/plot"
	influxdb "github.com/influxdb/influxdb/client"
)

// InfluxDBConnector represents the main structure of the InfluxDB connector.
type InfluxDBConnector struct {
	name     string
	host     string
	useTLS   bool
	username string
	password string
	database string
	client   *influxdb.Client
	re       *regexp.Regexp
	series   map[string]map[string][2]string
}

func init() {
	Connectors["influxdb"] = func(name string, settings map[string]interface{}) (Connector, error) {
		var (
			pattern string
			err     error
		)

		connector := &InfluxDBConnector{
			name:     name,
			host:     "localhost:8086",
			username: "root",
			password: "root",
			series:   make(map[string]map[string][2]string),
		}

		if connector.host, err = config.GetString(settings, "host", false); err != nil {
			return nil, err
		}

		if connector.useTLS, err = config.GetBool(settings, "use_tls", false); err != nil {
			return nil, err
		}

		if connector.username, err = config.GetString(settings, "username", false); err != nil {
			return nil, err
		}

		if connector.password, err = config.GetString(settings, "password", false); err != nil {
			return nil, err
		}

		if connector.database, err = config.GetString(settings, "database", true); err != nil {
			return nil, err
		}

		if pattern, err = config.GetString(settings, "pattern", true); err != nil {
			return nil, err
		}

		// Check and compile regexp pattern
		if connector.re, err = compilePattern(pattern); err != nil {
			return nil, fmt.Errorf("unable to compile regexp pattern: %s", err)
		}

		connector.client, err = influxdb.NewClient(&influxdb.ClientConfig{
			Host:     connector.host,
			Username: connector.username,
			Password: connector.password,
			Database: connector.database,
			IsSecure: connector.useTLS,
		})

		if err != nil {
			return nil, fmt.Errorf("unable to create client: %s", err)
		}

		return connector, nil
	}
}

// GetName returns the name of the current connector.
func (connector *InfluxDBConnector) GetName() string {
	return connector.name
}

// GetPlots retrieves time series data from provider based on a query and a time interval.
func (connector *InfluxDBConnector) GetPlots(query *plot.Query) ([]*plot.Series, error) {
	l := len(query.Series)
	if l == 0 {
		return nil, fmt.Errorf("influxdb[%s]: requested series list is empty", connector.name)
	}

	metrics := make([]string, l)
	columns := make([]string, l)
	for i, series := range query.Series {
		metrics[i] = strconv.Quote(connector.series[series.Source][series.Metric][0])
		columns[i] = strconv.Quote(connector.series[series.Source][series.Metric][1])
	}

	influxdbQuery := fmt.Sprintf(
		"select %s from %s where time > %ds and time < %ds order asc",
		strings.Join(columns, ","),
		strings.Join(metrics, ","),
		query.StartTime.Unix(),
		query.EndTime.Unix(),
	)

	q, err := connector.client.Query(influxdbQuery, "s")
	if err != nil {
		return nil, fmt.Errorf("influxdb[%s]: unable to perform query: %s", connector.name, err)
	}

	results := make([]*plot.Series, 0)

	seriesMap := make(map[string]*plot.Series)

	step := int(query.EndTime.Sub(query.StartTime) / time.Duration(query.Sample))

	for _, r := range q {
		name := r.GetName()
		columns := r.GetColumns()[2:]

		for i := range columns {
			seriesMap[name+"\x1e"+columns[i]] = &plot.Series{
				Step: step,
			}
		}

		for _, point := range r.GetPoints() {
			for i := 0; i < len(columns); i++ {
				key := name + "\x1e" + columns[i]
				seriesMap[key].Plots = append(seriesMap[key].Plots, plot.Plot{
					Time:  time.Unix(int64(point[0].(float64)), 0),
					Value: plot.Value(point[i+2].(float64)),
				})
			}
		}
	}

	for _, s := range query.Series {
		key := connector.series[s.Source][s.Metric][0] + "\x1e" + connector.series[s.Source][s.Metric][1]
		if _, ok := seriesMap[key]; !ok {
			continue
		}

		entry := seriesMap[key]
		entry.Name = s.Name

		results = append(results, entry)
	}

	return results, nil
}

// Refresh triggers a full connector data update.
func (connector *InfluxDBConnector) Refresh(originName string, outputChan chan<- *catalog.Record) error {
	seriesList, err := connector.client.Query("select * from /.*/ limit 1")
	if err != nil {
		return fmt.Errorf("influxdb[%s]: unable to fetch series list: %s", connector.name, err)
	}

	for _, series := range seriesList {
		var seriesName, sourceName, metricName string

		seriesName = series.GetName()

		seriesPoints := series.GetPoints()
		if len(seriesPoints) == 0 {
			logger.Log(logger.LevelInfo,
				"connector",
				"influxdb[%s]: series `%s' does not return sample data, ignoring",
				connector.name,
				seriesName,
			)
			continue
		}

		for columnIndex, columnName := range series.GetColumns() {
			if columnName == "time" || columnName == "sequence_number" {
				continue
			} else if _, ok := seriesPoints[0][columnIndex].(float64); !ok {
				continue
			}

			seriesMatch, err := matchSeriesPattern(connector.re, seriesName+"."+columnName)
			if err != nil {
				logger.Log(logger.LevelInfo,
					"connector",
					"influxdb[%s]: series `%s' does not match pattern, ignoring",
					connector.name,
					seriesName,
				)
				continue
			}

			sourceName, metricName = seriesMatch[0], seriesMatch[1]

			if _, ok := connector.series[sourceName]; !ok {
				connector.series[sourceName] = make(map[string][2]string)
			}

			connector.series[sourceName][metricName] = [2]string{seriesName, columnName}

			outputChan <- &catalog.Record{
				Origin:    originName,
				Source:    sourceName,
				Metric:    metricName,
				Connector: connector,
			}
		}
	}

	return nil
}
