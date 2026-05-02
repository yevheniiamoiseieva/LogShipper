FROM grafana/grafana:11.1.0

ENV GF_AUTH_ANONYMOUS_ENABLED=true
ENV GF_AUTH_ANONYMOUS_ORG_ROLE=Admin
ENV GF_AUTH_DISABLE_LOGIN_FORM=true
ENV GF_DASHBOARDS_DEFAULT_HOME_DASHBOARD_PATH=/var/lib/grafana/dashboards/pipeline.json
ENV GF_SERVER_HTTP_PORT=3000

COPY provisioning /etc/grafana/provisioning
COPY dashboards /var/lib/grafana/dashboards
