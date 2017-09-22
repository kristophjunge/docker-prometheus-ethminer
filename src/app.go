package main

import (
    "io"
    "net/http"
    "log"
    "os"
    "strconv"
    "regexp"
    "time"
    "errors"
    "github.com/rogpeppe/rog-go/reverse"
)

// Constants
const MAX_LINES = 100
const MAX_DELAY = 120 // Lets try two minutes to avoids being reported as down when switching from 23:59 to 00:00

// Configuration variables
var logPath string
var minerId string
var timeZone string
var timeZoneLocation *time.Location

func floatToString(value float64, precision int) string {
    return strconv.FormatFloat(value, 'f', precision, 64)
}

func integerToString(value int) string {
    return strconv.Itoa(value)
}

func stringToFloat(value string) float64 {
    result, err := strconv.ParseFloat(value, 64)
    if err != nil {
        log.Fatal(err)
    }
    return result
}

func stringToInteger(value string) int {
    result, err := strconv.Atoi(value)
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

func parseHashRate(line string) (float64, error) {
    expression := regexp.MustCompile("\\s?([\\d.]+)\\s?MH/s\\s?")
    match := expression.FindStringSubmatch(line)
    if (len(match) > 0) {
        return stringToFloat(match[1]), nil
    }
    return 0, errors.New("Could not parse hash rate ")
}

func parseTime(line string) (int, int, int, error) {
    expression := regexp.MustCompile("([\\d]{2,2}):([\\d]{2,2}):([\\d]{2,2})")
    match := expression.FindStringSubmatch(line)
    if (len(match) > 0) {
        return stringToInteger(match[1]), stringToInteger(match[2]), stringToInteger(match[3]), nil
    }
    return 0, 0, 0, errors.New("Could not parse time ")
}

func metrics(w http.ResponseWriter, r *http.Request) {
    log.Print("Serving /metrics")

    var err error
    var up int = 0
    var readLines int
    var hashRate float64
    var logHour int
    var logMinute int
    var logSecond int
    var logTime time.Time

    // Open log file
    file, err := os.Open(logPath)
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    // Create a new scanner and read the file line by line in reverse order
    scanner := reverse.NewScanner(file)

    for scanner.Scan() {
        line := scanner.Text()
        //log.Println(line)

        // Exit if we are reading too many lines without finding a valid one
        readLines++
        if (readLines > MAX_LINES) {
            up = 0
            log.Println("No valid log line found in the last " + integerToString(MAX_LINES) + " lines, exiting")
            break
        }

        // Parse hash rate
        hashRate, err = parseHashRate(line)
        if err != nil {
            continue
        }
        //log.Print(hashRate)

        // Parse time
        logHour, logMinute, logSecond, err = parseTime(line)
        if err != nil {
            continue
        }
        //log.Print(logHour)

        // Combine current date with log time
        now := time.Now().In(timeZoneLocation)
        logTime = time.Date(
            now.Year(),
            now.Month(),
            now.Day(),
            logHour,
            logMinute,
            logSecond,
            0,
            timeZoneLocation,
        )
        //log.Print(now)
        //log.Print(logTime)

        delta := now.Sub(logTime)
        seconds := delta.Seconds()
        //log.Print(seconds)

        // Check if last log message is older then allowed
        if (seconds > MAX_DELAY || seconds < 0) {
            log.Print("Last message from " + logTime.Format(time.RFC3339) + " is " + floatToString(seconds, 0) + " seconds before/after current time " + now.Format(time.RFC3339) + " which is above the allowed limit of " + integerToString(MAX_DELAY) + " seconds, miner is inactive")
            break;
        }

        log.Print("Miner is active with a hashrate of " + floatToString(hashRate, 2) + "MH/s")

        up = 1

        break;
    }

    if err = scanner.Err(); err != nil {
        log.Fatal(err)
    }

    io.WriteString(w, formatValue("ethminer_up", "miner=\"" + minerId + "\"", integerToString(up)))
    if (up == 1) {
        io.WriteString(w, formatValue("ethminer_hashrate", "miner=\"" + minerId + "\"", floatToString(hashRate, 2)))
    }
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
    var err error

    logPath = "/var/log/ethminer.log"
    log.Print("Monitoring logfile: " + logPath)

    minerId = os.Getenv("MINER_ID")
    if minerId == "" {
        minerId = "default"
    }
    log.Print("Serving stats with miner id: " + minerId)

    timeZone = os.Getenv("TIME_ZONE")
    if timeZone == "" {
        timeZone = "Europe/Berlin"
    }
    timeZoneLocation, err = time.LoadLocation(timeZone)
    if err != nil {
        panic(err)
    }
    log.Print("Using timezone: " + timeZone + ", Current time: " + time.Now().In(timeZoneLocation).Format(time.RFC3339))

    log.Print("Ethminer exporter running")
    http.HandleFunc("/", index)
    http.HandleFunc("/metrics", metrics)
    http.ListenAndServe(":9201", nil)
}
