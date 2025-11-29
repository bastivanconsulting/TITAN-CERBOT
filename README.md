# Titan Certbot

Titan Certbot is a lightweight utility written in Go designed to automate the retrieval and renewal of Let's Encrypt SSL/TLS certificates for the **Titan Web Server**. It leverages the [Lego](https://github.com/go-acme/lego) library to handle the ACME protocol using the HTTP-01 challenge.

## Features

-   **Automated Certificate Retrieval**: Fetches certificates from Let's Encrypt.
-   **HTTP-01 Challenge**: Uses the webroot method to validate domain ownership.
-   **Automatic Installation**: Saves the generated `.crt` and `.key` files directly to the directory monitored by Titan.
-   **Service Restart**: Automatically attempts to restart the `titan-server` service using `systemctl` to apply the new certificates.
-   **Staging Support**: Includes a flag to use the Let's Encrypt staging environment for testing.

## Prerequisites

-   **Go** (1.24 or later) installed to build the project.
-   **Titan Web Server** running and serving files from the specified webroot.
-   **Systemd**: The tool attempts to restart the server using `systemctl`.

## Installation

Clone the repository and build the binary:

```bash
git clone git@github.com:bastivanconsulting/TITAN-CERBOT.git
cd titan-certbot
go build -o titan-certbot
```

## Usage

Run the tool with root privileges (usually required to write to `/etc/titan` and restart services).

```bash
sudo ./titan-certbot -domain <your-domain> -email <your-email> [options]
```

### Command Line Arguments

| Flag | Description | Default |
| :--- | :--- | :--- |
| `-domain` | **Required**. The domain name to certify (e.g., `example.com`). | |
| `-email` | **Required**. Email address for Let's Encrypt registration and recovery. | |
| `-webroot`| The root directory of your website. Let's Encrypt will check for the challenge token here. | `/var/www/titan/public` |
| `-out` | The destination directory where the certificate and key will be saved. | `/etc/titan/certs/manual` |
| `-staging`| Use the Let's Encrypt Staging environment (for testing purposes). | `false` |

### Example

To obtain a production certificate for `example.com`:

```bash
sudo ./titan-certbot -domain example.com -email admin@example.com
```

To test with the Staging environment (avoids rate limits):

```bash
sudo ./titan-certbot -domain example.com -email admin@example.com -staging
```

## How It Works

1.  **Initialization**: Generates an ephemeral private key for the ACME account.
2.  **Configuration**: Sets up the Lego client (Staging or Production CA).
3.  **Challenge Setup**: Configures the HTTP-01 provider. The tool writes the ACME challenge token to `.well-known/acme-challenge/` inside the specified `-webroot`.
4.  **Registration**: Registers the account with Let's Encrypt.
5.  **Certificate Request**: Requests the certificate for the domain.
6.  **Storage**: Saves the certificate (`<domain>.crt`) and private key (`<domain>.key`) to the `-out` directory.
7.  **Restart**: Executes `systemctl restart titan-server` to reload the web server with the new certificate.

## Troubleshooting

-   **Verification Failure**: Ensure your web server is running and accessible on port 80. The directory specified by `-webroot` must be served publicly.
-   **Permission Denied**: Run the tool with `sudo` so it can write to `/etc/titan/certs` and restart the service.
-   **Restart Failed**: If the automatic restart fails, you will see a warning. In this case, restart the Titan server manually.
