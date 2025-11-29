# Titan Certbot

Titan Certbot is a lightweight utility written in Go designed to automate the retrieval and renewal of Let's Encrypt SSL/TLS certificates for the **Titan Web Server**. It leverages the [Lego](https://github.com/go-acme/lego) library to handle the ACME protocol using the HTTP-01 challenge.

## Features

-   **Multi-Domain Support**: Manage multiple domains via a configuration file.
-   **Automated Renewal**: Automatically checks expiration dates and only renews certificates when they are within 30 days of expiration.
-   **HTTP-01 Challenge**: Uses the webroot method to validate domain ownership.
-   **Automatic Installation**: Saves the generated `.crt` and `.key` files directly to the directory monitored by Titan.
-   **Configurable Restart**: Automatically executes a custom command (e.g., `systemctl restart titan-server`) to apply new certificates.
-   **Cross-Platform**: Builds for Linux and Windows.
-   **Staging Support**: Includes an option to use the Let's Encrypt staging environment for testing.

## Prerequisites

-   **Go** (1.24 or later) installed to build the project.
-   **Titan Web Server** running and serving files from the specified webroot.

## Installation

### Using Packages (Linux)

Download the `.deb` or `.rpm` package from the [Releases](https://github.com/bastivanconsulting/TITAN-CERBOT/releases) page.

**Debian/Ubuntu:**
```bash
sudo dpkg -i titan-certbot_*.deb
```

**RHEL/CentOS:**
```bash
sudo rpm -ivh titan-certbot-*.rpm
```

The package installs:
- Binary to `/usr/bin/titan-certbot`
- Example config to `/etc/titan/certbot-config.example.json`
- Systemd timer and service (active by default, runs daily)

### Manual Installation

Clone the repository and build the binary:

```bash
git clone git@github.com:bastivanconsulting/TITAN-CERBOT.git
cd titan-certbot
go build -o titan-certbot
```

## Usage

### Configuration File

1.  Copy the example config:
    ```bash
    sudo cp /etc/titan/certbot-config.example.json /etc/titan/certbot-config.json
    ```
2.  Edit `/etc/titan/certbot-config.json` to manage your domains:

```json
{
  "email": "admin@example.com",
  "staging": false,
  "restart_command": "systemctl restart titan-server",
  "domains": [
    {
      "domain": "example.com",
      "webroot": "/var/www/html",
      "output_dir": "/etc/titan/certs"
    }
  ]
}
```

### Automation (Cronjob/Systemd)

If you installed via `.deb` or `.rpm`, a systemd timer is already enabled to run daily. It executes:
```bash
/usr/bin/titan-certbot -config /etc/titan/certbot-config.json
```

To verify the timer status:
```bash
systemctl status titan-certbot.timer
```

To run manually:
```bash
sudo titan-certbot -config /etc/titan/certbot-config.json
```

### Legacy Command Line Arguments

For single domain issuance without a config file:

```bash
sudo titan-certbot -domain <your-domain> -email <your-email> [options]
```

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-config` | Path to the JSON configuration file. | |
| `-domain` | The domain name to certify (e.g., `example.com`). | |
| `-email` | Email address for Let's Encrypt registration and recovery. | |
| `-webroot`| The root directory of your website. Let's Encrypt will check for the challenge token here. | `/var/www/titan/public` |
| `-out` | The destination directory where the certificate and key will be saved. | `/etc/titan/certs/manual` |
| `-staging`| Use the Let's Encrypt Staging environment (for testing purposes). | `false` |

## How It Works

1.  **Check Expiration**: It scans existing certificates in the output directory. If a certificate expires in less than 30 days (or is missing), it proceeds to renewal.
2.  **Challenge Setup**: Configures the HTTP-01 provider. The tool writes the ACME challenge token to `.well-known/acme-challenge/` inside the specified `-webroot`.
3.  **Certificate Request**: Requests the certificate for the domain via Let's Encrypt (Lego).
4.  **Storage**: Saves the certificate (`<domain>.crt`) and private key (`<domain>.key`) to the output directory.
5.  **Restart**: If at least one certificate was renewed, it executes the `restart_command` to reload the web server.

## Troubleshooting

-   **Verification Failure**: Ensure your web server is running and accessible on port 80. The directory specified by `-webroot` must be served publicly.
-   **Permission Denied**: Run the tool with `sudo` so it can write to destination directories and restart services.
-   **Restart Failed**: Check the `restart_command` in your config. On Windows, you might use a batch script or `net stop/start`.
