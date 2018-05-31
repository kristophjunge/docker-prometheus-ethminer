# Docker Prometheus Ethminer Exporter

Dockerized Prometheus exporter to monitor ethminer log output written in Go.

Exports status status and hashrate.

Example output:

```
ethminer_up{miner="default"} 1
ethminer_hashrate{miner="default"} 188.74
```
