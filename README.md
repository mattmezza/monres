# monres - Lightweight VPS Resource Monitor

<div align="center">
    <img src="https://raw.githubusercontent.com/mattmezza/monres/main/icon.png" alt="Monres logo" width="175px" height="175px">
</div>

monres is a simple, lightweight, and easy-to-install software tool for monitoring
core system resources (CPU, Memory, Disk I/O, Network I/O) on a linux VPS.
It runs as a background service, triggers alerts based on user-defined thresholds,
and sends notifications via Email and Telegram.

## Features

- Monitors CPU, Memory, Disk I/O, Network I/O.
- Direct OS metric collection (reads `/proc`, `/sys`).
- Configurable alert rules (threshold, duration, aggregation).
- Notifications via Email (SMTP) and Telegram.
- Customizable notification templates.
- Sensitive credentials read from environment variables.
- Designed for minimal resource consumption.
- Single binary deployment (Go).
- systemd service for management.

## Installation

### Prerequisites

- Go (latest stable version, for building)
- A linux VPS (tested on Ubuntu)

### Building from Source

1.  Clone the repository:
    ```bash
    git clone [https://github.com/mattmezza/monres.git](https://github.com/mattmezza/monres.git)
    cd monres
    ```

2.  Build the binary:
    ```bash
    go build -ldflags="-s -w" -o monres cmd/monres/main.go
    ```
    The `-s -w` flags strip debug information and symbol table, reducing binary size.

3.  Copy the binary to a suitable location:
    ```bash
    sudo cp monres /usr/local/bin/
    ```

For your convenience, a Makefile is provided.


### Configuration

1.  Create the configuration directory:
    ```bash
    sudo mkdir -p /etc/monres
    ```

2.  Copy the example configuration:
    ```bash
    sudo cp config.example.yaml /etc/monres/config.yaml
    ```

3.  Edit `/etc/monres/config.yaml` to suit your needs. Refer to the comments
    within the file and the "Configuration Details" section below.

4.  Create the environment file for sensitive credentials:
    ```bash
    sudo touch /etc/monres/monres.env
    sudo chown monres:monres /etc/monres/monres.env # Assuming 'monres' user/group
    sudo chmod 600 /etc/monres/monres.env
    ```
    Edit `/etc/monres/monres.env` and add your secrets, for example:
    ```ini
    MONRES_SMTP_PASSWORD_EMAIL="your_smtp_password"
    MONRES_TELEGRAM_TOKEN_TELEGRAM="your_telegram_bot_token"
    ```
    The environment variable names are constructed as
    `MONRES_<SENSITIVE_FIELD_UPPERCASE>_<CHANNEL_NAME_UPPERCASE_UNDERSCORED>`.
    For example, for a channel named `my-email-channel` and field
    `smtp_password`, the env var would be
    `MONRES_SMTP_PASSWORD_MY_EMAIL_CHANNEL`.

### Setup as a Systemd Service

1.  Create a dedicated system user for monres:
    ```bash
    sudo groupadd --system monres
    sudo useradd --system --gid monres --shell /sbin/nologin --home-dir /var/lib/monres monres
    sudo chown -R monres:monres /var/lib/monres
    # Grant read access to config for the monres user
    sudo chown root:monres /etc/monres # dir owned by root, group readable by monres
    sudo chmod 750 /etc/monres
    sudo chown root:monres /etc/monres/config.yaml
    sudo chmod 640 /etc/monres/config.yaml
    # Ensure monres.env is also correctly permissioned and owned as above
    sudo chown monres:monres /etc/monres/monres.env
    sudo chmod 600 /etc/monres/monres.env
    ```

2.  Copy the systemd service file:
    ```bash
    sudo cp deploy/systemd/monres.service /etc/systemd/system/
    ```

3.  Reload systemd, enable, and start the service:
    ```bash
    sudo systemctl daemon-reload
    sudo systemctl enable monres.service
    sudo systemctl start monres.service
    ```

4.  Check the status:
    ```bash
    sudo systemctl status monres.service
    journalctl -u monres -f
    ```

## Configuration Details

- `interval_seconds`: The interval in seconds at which metrics are collected
  and alerts are evaluated. Default is `1` (every second).
- `hostname`: The hostname of the VPS, used in notifications.
  Default is the system's hostname.
- `alerts`: A list of alert configurations. Each alert has:
  - `name`: Unique identifier for the alert.
  - `metric`: The metric to monitor (e.g., `cpu_percent_total`). See below for
    the full list of metrics.
  - `threshold`: The threshold value that triggers the alert.
  - `condition`: The operator for the threshold condition
    (i.e. `>`, `<`, `>=`, `<=`).
  - `duration`: The duration over which the metric must exceed the threshold to
    trigger the alert.
  - `aggregation`: How to aggregate the metric values (i.e. `avg`, `max`).
  - `channels`: List of channels to notify when the alert is triggered.
- `notification_channels`: A list of notification channels. Each channel has:
    - `type`: The type of channel (i.e. `email`, `telegram`, `stdout`).
    - `name`: Unique identifier for the channel. This is used to reference the
      channel in the alerts configuration.
    - `config`: Configuration specific to the channel type (e.g., SMTP settings
      for email, bot token for Telegram).
- `templates`: Customizable notification templates for each alert state (fired
  or resolved). Each template can include placeholders for dynamic content
  (e.g., `{{ .AlertName }}`, `{{ .MetricValue }}`). See the example config.

## Metrics Collected

-   `cpu_percent_total`: Total CPU usage percentage.
-   `mem_percent_used`: Used memory percentage (based on MemAvailable).
-   `mem_percent_free`: Free memory percentage (based on MemAvailable).
-   `swap_percent_used`: Used swap percentage.
-   `swap_percent_free`: Free swap percentage.
-   `disk_read_bytes_ps`: Aggregated disk read bytes per second.
-   `disk_write_bytes_ps`: Aggregated disk write bytes per second.
-   `net_recv_bytes_ps`: Aggregated network received bytes per second.
-   `net_sent_bytes_ps`: Aggregated network transmitted bytes per second.
