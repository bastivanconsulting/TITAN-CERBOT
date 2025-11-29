package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/http/webroot"
	"github.com/go-acme/lego/v4/registration"
)

// Config represents the configuration file structure
type Config struct {
	Email          string       `json:"email"`
	Domains        []DomainConf `json:"domains"`
	Staging        bool         `json:"staging"`
	RestartCommand string       `json:"restart_command"`
}

type DomainConf struct {
	Domain    string `json:"domain"`
	Webroot   string `json:"webroot"`
	OutputDir string `json:"output_dir"`
}

// User represents the ACME user required by Lego
type User struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *User) GetEmail() string { return u.Email }
func (u *User) GetRegistration() *registration.Resource { return u.Registration }
func (u *User) GetPrivateKey() crypto.PrivateKey { return u.key }

func main() {
	// --- Command Line Arguments ---
	configPath := flag.String("config", "", "Path to configuration file (JSON)")
	
	// Legacy flags
	domain := flag.String("domain", "", "Domain to certify")
	email := flag.String("email", "", "Email for Let's Encrypt")
	webrootDir := flag.String("webroot", "/var/www/titan/public", "Webroot directory")
	outDir := flag.String("out", "/etc/titan/certs/manual", "Output directory")
	staging := flag.Bool("staging", false, "Use staging environment")

	flag.Parse()

	var config Config

	// Load config from file if provided
	if *configPath != "" {
		file, err := os.ReadFile(*configPath)
		if err != nil {
			log.Fatalf("Error reading config file: %v", err)
		}
		if err := json.Unmarshal(file, &config); err != nil {
			log.Fatalf("Error parsing config file: %v", err)
		}
	} else if *domain != "" && *email != "" {
		// Fallback to flags
		config = Config{
			Email: *email,
			Domains: []DomainConf{
				{
					Domain:    *domain,
					Webroot:   *webrootDir,
					OutputDir: *outDir,
				},
			},
			Staging:        *staging,
			RestartCommand: "systemctl restart titan-server",
		}
	} else {
		fmt.Println("Usage: ./titan-certbot -config <config.json>")
		fmt.Println("   OR: ./titan-certbot -domain <domain> -email <email> [options]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	processConfig(config)
}

func processConfig(config Config) {
	renewed := false

	// 1. Initialize User and Client
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Error generating private key: %v", err)
	}

	myUser := User{
		Email: config.Email,
		key:   privateKey,
	}

	legoConfig := lego.NewConfig(&myUser)

	if config.Staging {
		legoConfig.CADirURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
		log.Println("⚠️  STAGING mode active")
	} else {
		legoConfig.CADirURL = "https://acme-v02.api.letsencrypt.org/directory"
		log.Println("🔒 PRODUCTION mode active")
	}
	legoConfig.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(legoConfig)
	if err != nil {
		log.Fatalf("Error creating client: %v", err)
	}

	// 2. Register Account
	log.Println("👤 Registering ACME account...")
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		log.Fatalf("Error registering account: %v", err)
	}
	myUser.Registration = reg

	// 3. Process each domain
	for _, d := range config.Domains {
		if shouldRenew(d) {
			if obtainCertificate(client, d) {
				renewed = true
			}
		} else {
			log.Printf("✅ Certificate for %s is valid. Skipping.", d.Domain)
		}
	}

	// 4. Restart Server if needed
	if renewed {
		if config.RestartCommand != "" {
			restartServer(config.RestartCommand)
		} else {
			log.Println("ℹ️  No restart command specified.")
		}
	} else {
		log.Println("✨ No renewals performed.")
	}
}

func shouldRenew(d DomainConf) bool {
	certPath := filepath.Join(d.OutputDir, d.Domain+".crt")

	// If file doesn't exist, we must renew
	data, err := os.ReadFile(certPath)
	if os.IsNotExist(err) {
		log.Printf("⚠️  Certificate missing for %s", d.Domain)
		return true
	} else if err != nil {
		log.Printf("Error reading certificate %s: %v", certPath, err)
		return true // Fail safe: try to renew
	}

	block, _ := pem.Decode(data)
	if block == nil {
		log.Printf("❌ Failed to parse PEM block for %s", d.Domain)
		return true
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Printf("❌ Failed to parse certificate for %s: %v", d.Domain, err)
		return true
	}

	// Check if expires within 30 days
	daysLeft := time.Until(cert.NotAfter).Hours() / 24
	if daysLeft <= 30 {
		log.Printf("⚠️  Certificate for %s expires in %.1f days. Renewing...", d.Domain, daysLeft)
		return true
	}

	log.Printf("ℹ️  Certificate for %s expires in %.1f days.", d.Domain, daysLeft)
	return false
}

func obtainCertificate(client *lego.Client, d DomainConf) bool {
	// Configure Webroot Challenge for this domain
	provider, err := webroot.NewHTTPProvider(d.Webroot)
	if err != nil {
		log.Printf("❌ Error creating webroot provider for %s: %v", d.Domain, err)
		return false
	}

	err = client.Challenge.SetHTTP01Provider(provider)
	if err != nil {
		log.Printf("❌ Error setting HTTP provider for %s: %v", d.Domain, err)
		return false
	}

	log.Printf("🚀 Requesting certificate for %s...", d.Domain)
	request := certificate.ObtainRequest{
		Domains: []string{d.Domain},
		Bundle:  true,
	}
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		log.Printf("❌ Failed to obtain certificate for %s: %v", d.Domain, err)
		return false
	}

	// Save files
	if err := os.MkdirAll(d.OutputDir, 0755); err != nil {
		log.Printf("❌ Error creating output dir %s: %v", d.OutputDir, err)
		return false
	}

	certPath := filepath.Join(d.OutputDir, d.Domain+".crt")
	keyPath := filepath.Join(d.OutputDir, d.Domain+".key")

	if err := os.WriteFile(certPath, certificates.Certificate, 0644); err != nil {
		log.Printf("❌ Error writing cert for %s: %v", d.Domain, err)
		return false
	}
	if err := os.WriteFile(keyPath, certificates.PrivateKey, 0600); err != nil {
		log.Printf("❌ Error writing key for %s: %v", d.Domain, err)
		return false
	}

	log.Printf("✅ Certificate saved for %s", d.Domain)
	return true
}

func restartServer(command string) {
	log.Printf("🔄 Executing restart command: %s", command)
	
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		log.Printf("⚠️  Restart command failed: %v", err)
		if len(output) > 0 {
			log.Printf("   Output: %s", string(output))
		}
	} else {
		log.Println("✅ Server restarted successfully!")
	}
}
