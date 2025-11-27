package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/providers/http/webroot"
	"github.com/go-acme/lego/v4/registration"
)

// User représente l'utilisateur ACME requis par la librairie Lego
type User struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

func (u *User) GetEmail() string { return u.Email }
func (u *User) GetRegistration() *registration.Resource { return u.Registration }
func (u *User) GetPrivateKey() crypto.PrivateKey { return u.key }

func main() {
	// --- Arguments de la ligne de commande ---
	domain := flag.String("domain", "", "Domaine à certifier (ex: mon-site.com)")
	email := flag.String("email", "", "Email pour l'enregistrement Let's Encrypt")
	webrootDir := flag.String("webroot", "/var/www/titan/public", "Dossier racine du site (où Titan peut lire les fichiers)")
	outDir := flag.String("out", "/etc/titan/certs/manual", "Dossier de destination (Titan scanne ce dossier)")
	staging := flag.Bool("staging", false, "Utiliser l'environnement de test (Staging)")
	
	flag.Parse()

	if *domain == "" || *email == "" {
		fmt.Println("❌ Usage : ./titan-certbot -domain <domaine> -email <email> [options]")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// 1. Initialisation de la clé privée éphémère pour le compte ACME
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Erreur génération clé : %v", err)
	}

	myUser := User{
		Email: *email,
		key:   privateKey,
	}

	// 2. Configuration du client Lego
	config := lego.NewConfig(&myUser)

	if *staging {
		config.CADirURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
		log.Println("⚠️  Mode STAGING actif (Certificat de test)")
	} else {
		config.CADirURL = "https://acme-v02.api.letsencrypt.org/directory"
		log.Println("🔒 Mode PRODUCTION actif")
	}
	config.Certificate.KeyType = certcrypto.EC256

	client, err := lego.NewClient(config)
	if err != nil {
		log.Fatalf("Erreur création client : %v", err)
	}

	// 3. Configuration du Challenge Webroot
	// L'outil va écrire le token dans *webrootDir/.well-known/acme-challenge/
	provider, err := webroot.NewHTTPProvider(*webrootDir)
	if err != nil {
		log.Fatalf("Erreur provider webroot : %v", err)
	}

	err = client.Challenge.SetHTTP01Provider(provider)
	if err != nil {
		log.Fatal(err)
	}

	// 4. Enregistrement du compte
	log.Println("👤 Enregistrement du compte ACME...")
	reg, err := client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
	if err != nil {
		log.Fatalf("Erreur enregistrement : %v", err)
	}
	myUser.Registration = reg

	// 5. Demande du certificat
	log.Printf("🚀 Demande de certificat pour %s...", *domain)
	request := certificate.ObtainRequest{
		Domains: []string{*domain},
		Bundle:  true,
	}
	certificates, err := client.Certificate.Obtain(request)
	if err != nil {
		log.Fatalf("❌ Échec de l'obtention du certificat : %v", err)
	}

	// 6. Sauvegarde des fichiers
	if err := os.MkdirAll(*outDir, 0755); err != nil {
		log.Fatal(err)
	}

	certPath := filepath.Join(*outDir, *domain+".crt")
	keyPath := filepath.Join(*outDir, *domain+".key")

	// Écriture des fichiers (644 pour crt, 600 pour key)
	if err := os.WriteFile(certPath, certificates.Certificate, 0644); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(keyPath, certificates.PrivateKey, 0600); err != nil {
		log.Fatal(err)
	}

	log.Printf("✅ Certificats sauvegardés dans : %s", *outDir)

	// 7. Redémarrage du service Titan
	restartTitan()
}

func restartTitan() {
	log.Println("🔄 Redémarrage de Titan Web Server...")
	
	// Tentative via systemctl
	cmd := exec.Command("systemctl", "restart", "titan-server")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		log.Printf("⚠️  Échec du redémarrage automatique via systemd : %v", err)
		if len(output) > 0 {
			log.Printf("   Sortie : %s", string(output))
		}
		log.Println("👉 Veuillez redémarrer Titan manuellement pour prendre en compte le certificat.")
	} else {
		log.Println("✅ Titan Server redémarré avec succès !")
	}
}
