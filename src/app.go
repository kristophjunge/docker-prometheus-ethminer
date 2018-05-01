package main

import (
    "io"
    "net/http"
    "log"
    "os"
    "strconv"
    "io/ioutil"
    "encoding/json"
    "net"
    "strings"
)

const LISTEN_ADDRESS = ":9201"

var apiUrl string
var minerId string
var testMode string

type EthminerStatistics struct {
    ID int64 `json:"id"`
    JSONRPC string `json:"jsonrpc"`
    Result []string `json:"result"`
}

func stringToInteger(value string) int64 {
    if value == "" {
        return 0
    }
    result, err := strconv.ParseInt(value, 10, 64)
    if err != nil {
        log.Fatal(err)
    }
    return result
}

func integerToString(value int64) string {
    return strconv.FormatInt(value, 10)
}

func floatToString(value float64, precision int64) string {
    return strconv.FormatFloat(value, 'f', int(precision), 64)
}

func stringToFloat(value string) float64 {
    if value == "" {
        return 0
    }
    result, err := strconv.ParseFloat(value, 64)
    if err != nil {
        log.Fatal(err)
    }
    return result
}

func formatValue(key string, meta string, value string) string {
    result := key;
    if (meta != "") {
        result += "{" + meta + "}";
    }
    result += " "
    result += value
    result += "\n"
    return result
}

const StopCharacter = "\r\n\r\n"

func queryData() (string, error) {
    var err error

    message := "{\"method\":\"miner_getstat1\",\"jsonrpc\":\"2.0\",\"id\":5}"

	conn, err := net.Dial("tcp", apiUrl)

	if err != nil {
		return "", err
	}

    defer conn.Close()

	conn.Write([]byte(message))
	conn.Write([]byte(StopCharacter))

	buff := make([]byte, 1024)
	n, _ := conn.Read(buff)

    return string(buff[:n]), nil;
}

func getTestData() (string, error) {
    dir, err := os.Getwd()
    if err != nil {
        return "", err;
    }
    body, err := ioutil.ReadFile(dir + "/test.json")
    if err != nil {
        return "", err;
    }
    return string(body), nil
}

func metrics(w http.ResponseWriter, r *http.Request) {
    log.Print("Serving /metrics")

    var up int64 = 1
    var hashRate float64 = 0
    var jsonString string
    var err error

    if (testMode == "1") {
        jsonString, err = getTestData()
    } else {
        jsonString, err = queryData()
    }
    if err != nil {
        log.Print(err)
        up = 0
    } else {
        // Parse JSON
        jsonData := EthminerStatistics{}
        json.Unmarshal([]byte(jsonString), &jsonData)

        s := strings.Split(jsonData.Result[2], ";")
        hashRate = stringToFloat(s[0]) / 1000
    }

    // Output
    io.WriteString(w, formatValue("ethminer_up", "miner=\"" + minerId + "\"", integerToString(up)))
    io.WriteString(w, formatValue("ethminer_hashrate", "miner=\"" + minerId + "\"", floatToString(hashRate, 6)))
}

func index(w http.ResponseWriter, r *http.Request) {
    log.Print("Serving /index")
    html := string(`<!doctype html>
<html>
    <head>
        <meta charset="utf-8">
        <title>Ethminer Exporter</title>
    </head>
    <body>
        <h1>Ethminer Exporter</h1>
        <p><a href="/metrics">Metrics</a></p>
    </body>
</html>
`)
    io.WriteString(w, html)
}

func main() {
testMode = os.Getenv("TEST_MODE")
    if (testMode == "1") {
        log.Print("Test mode is enabled")
    }

    apiUrl = os.Getenv("API_URL")
    log.Print("API URL: " + apiUrl)

    minerId = os.Getenv("MINER_ID")
    log.Print("Miner ID: " + minerId)

    log.Print("Ethminer exporter listening on " + LISTEN_ADDRESS)
    http.HandleFunc("/", index)
    http.HandleFunc("/metrics", metrics)
    http.ListenAndServe(LISTEN_ADDRESS, nil)
}
