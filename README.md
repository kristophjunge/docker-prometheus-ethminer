# Docker Prometheus Ethminer Exporter

Dockerized Prometheus exporter to monitor ethminer log output written in Go.

Exports status up|down, timestamp of last activity and hashrate.

Example output:

```
ethminer_up{miner="default"} 1
ethminer_lastactivity{miner="default"} 1506727383
ethminer_hashrate{miner="default"} 188.74
```

The detection if the miner is active is done by checking that the last log time is not older than 60 seconds.

Works with large log files by reading the log file line by line from its end only until the necessary log line is found.


## Known Issues

Does not work if the log file only contains a single line.
This is a side effect of a special check that log lines are ignored when the preceding line contains "set work".
The reason is that the hash rate always decreases to around the half of its usual value when these lines occur.
To make monitoring and alerting possible its essential that the hash rate does spikes like caused by this behaviour.
