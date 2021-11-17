# mqtt2prom

Collect Sonoff mqtt Status Messages to provide them to prometheus

0.0.3
add retry on mqtt server connect

##BUILD
```
docker build -t jorie70/mqtt2prom:0.0.3 -t jorie70/mqtt2prom:latest .
```
## PUSH

```
docker push jorie70/mqtt2prom:latest
docker push jorie70/mqtt2prom:0.0.3
```
