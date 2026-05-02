FROM prom/prometheus:v2.53.0
COPY prometheus.yml /etc/prometheus/prometheus.yml
CMD ["--config.file=/etc/prometheus/prometheus.yml", "--storage.tsdb.retention.time=1h", "--web.listen-address=:9090"]
