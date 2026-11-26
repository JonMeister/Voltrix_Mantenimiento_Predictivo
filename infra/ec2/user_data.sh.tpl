#!/bin/bash
set -euxo pipefail

# 1. Habilitar 1 GiB de Swap persistente (obligatorio para EC2 de 1 GiB RAM)
if [ ! -f /swapfile ]; then
  fallocate -l 1G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  echo '/swapfile none swap sw 0 0' >> /etc/fstab
  sysctl vm.swappiness=20
  echo 'vm.swappiness=20' >> /etc/sysctl.conf
fi

# 2. Instalar Docker Engine y Docker Compose plugin
apt-get update -y
apt-get install -y ca-certificates curl git gnupg
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin
systemctl enable --now docker
usermod -aG docker ubuntu || true

# 3. Clonar repositorio del Equipo 6 y generar .env sin secretos en Git
APP_DIR="/opt/voltrix-mantenimiento"
rm -rf "$APP_DIR"
git clone --branch "${repo_branch}" "${repo_url}" "$APP_DIR"
cd "$APP_DIR"

cat > .env <<EOF
API_PORT=${api_port}
SERVICE_VERSION=1.0.0
LOG_LEVEL=info
MAX_CONCURRENT_JOBS=2
DEFAULT_WORKERS=2
RETRY_AFTER_SECONDS=2
DATA_SOURCE=${data_source}
UPSTREAM_TIMEOUT_SECONDS=3
URL_INVENTARIO=${url_inventario}
URL_ALERTAS=${url_alertas}
URL_REPORTES=${url_reportes}
URL_PRONOSTICOS=${url_pronosticos}
POSTGRES_DB=${postgres_db}
POSTGRES_USER=${postgres_user}
POSTGRES_PASSWORD=${postgres_password}
SEED_DEMO_DATA=true
EOF
chmod 600 .env

# 4. Descargar imagen desde Docker Hub y levantar API + PostgreSQL en red privada
docker compose pull
docker compose up -d

