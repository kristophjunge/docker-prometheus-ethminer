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

const LISTEN_ADDRESS = ":9201"
const MAX_LOG_LINES_TO_READ = 100
const MAX_LOG_MESSAGE_AGE_SECONDS = 60

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

func isSetWorkLine(line string) bool {
    expression := regexp.MustCompile("set work")
    return expression.MatchString(line)
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
    var line string
    var previousLine string

    // Read last log file modification time
    fileInfo, err := os.Stat(logPath)
    if err != nil {
        log.Fatal(err)
    }
    fileModifiedTime := fileInfo.ModTime().In(timeZoneLocation)
    // Use the file modified time as last log time if we don't find a time inside the log file.
    logTime = fileModifiedTime

    // Open log file
    file, err := os.Open(logPath)
    if err != nil {
        log.Fatal(err)
    }
    defer file.Close()

    // Create a new scanner and read the file line by line in reverse order
    scanner := reverse.NewScanner(file)

    for scanner.Scan() {
        // Always scan one line further to be able to detect decreasing hash rate reports after "set work" log lines.
        // 23:23:04|cudaminer6set work; seed: #f641c97b, target:#0000000112e0
        // 23:23:04|ethminer Mining on PoWhash #843ed6bf: 68.16MH/s [A611+3:R0+0:F0]
        // 23:23:04|ethminer Mining on PoWhash #843ed6bf: 178.26MH/s [A611+3:R0+0:F0]
        // This introduces the known issue that the log file parsing does not work when it only contains a single line.
        previousLine = line
        line = scanner.Text()
        //log.Println(line)

        // Exit if we are reading too many lines without finding a valid one
        readLines++
        if (readLines > MAX_LOG_LINES_TO_READ) {
            up = 0
            log.Println("No valid log line found in the last " + integerToString(MAX_LOG_LINES_TO_READ) + " lines, exiting")
            break
        }

        // Parse hash rate
        hashRate, err = parseHashRate(previousLine)
        if err != nil {
            continue
        }
        //log.Print(hashRate)

        // Parse time
        logHour, logMinute, logSecond, err = parseTime(previousLine)
        if err != nil {
            continue
        }
        //log.Print(logHour)

        // If the previous line is a set work line ignore current line
        // since it might contain a temporary decreasing hash rate.
        if isSetWorkLine(line) {
            log.Print("Skipping set work line")
            continue
        }

        // Combine log file modification date with log time from inside the log file
        logTime = time.Date(
            fileModifiedTime.Year(),
            fileModifiedTime.Month(),
            fileModifiedTime.Day(),
            logHour,
            logMinute,
            logSecond,
            0,
            timeZoneLocation,
        )
        //log.Print(now)
        //log.Print(logTime)

        now := time.Now().In(timeZoneLocation)
        delta := now.Sub(logTime)
        seconds := delta.Seconds()
        //log.Print(seconds)

        // Check if last log message is older then allowed
        if (seconds > MAX_LOG_MESSAGE_AGE_SECONDS || seconds < 0) {
            log.Print("Last message from " + logTime.Format(time.RFC3339) + " is " + floatToString(seconds, 0) + " seconds before/after current time " + now.Format(time.RFC3339) + " which is above the allowed limit of " + integerToString(MAX_LOG_MESSAGE_AGE_SECONDS) + " seconds, miner is inactive")
        } else {
            up = 1
        }

        break;
    }

    if err = scanner.Err(); err != nil {
        log.Fatal(err)
    }

    if up == 0 {
        hashRate = 0
    } else {
        log.Print("Miner is active with a hashrate of " + floatToString(hashRate, 2) + "MH/s")
    }

    io.WriteString(w, formatValue("ethminer_up", "miner=\"" + minerId + "\"", integerToString(up)))
    io.WriteString(w, formatValue("ethminer_lastactivity", "miner=\"" + minerId + "\"", strconv.FormatInt(logTime.Unix(), 10)))
    io.WriteString(w, formatValue("ethminer_hashrate", "miner=\"" + minerId + "\"", floatToString(hashRate, 2)))
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
        timeZone = "UTC"
    }
    timeZoneLocation, err = time.LoadLocation(timeZone)
    if err != nil {
        panic(err)
    }
    log.Print("Using timezone: " + timeZone + ", Current time: " + time.Now().In(timeZoneLocation).Format(time.RFC3339))

    log.Print("Ethminer exporter listening on " + LISTEN_ADDRESS)
    http.HandleFunc("/", index)
    http.HandleFunc("/metrics", metrics)
    http.ListenAndServe(LISTEN_ADDRESS, nil)
}
