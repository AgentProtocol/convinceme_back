package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize the ConvinceMe application",
	Long: `Initialize the ConvinceMe application by setting up required directories
and generating TLS certificates for secure communication.

This command will:
1. Create necessary directories (data, bin, tmp)
2. Generate secure TLS certificates for HTTPS
3. Set up proper file permissions
4. Create a template .env file if it doesn't exist`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Initializing ConvinceMe...")

		// Create necessary directories with proper permissions
		dirs := []string{"data", "bin", "tmp", "static/hls"}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Printf("Error creating directory %s: %v\n", dir, err)
				os.Exit(1)
			}
			fmt.Printf("✓ Created directory: %s\n", dir)
		}

		// Generate TLS certificates if they don't exist
		if _, err := os.Stat("cert.pem"); os.IsNotExist(err) {
			fmt.Println("\nGenerating TLS certificates...")

			// Create OpenSSL config file with enhanced settings
			opensslConfig := `[ req ]
default_bits = 2048
prompt = no
default_md = sha256
req_extensions = req_ext
distinguished_name = dn
[ dn ]
C = US
ST = California
L = San Francisco
O = ConvinceMe Development
OU = Development
CN = localhost
[ req_ext ]
subjectAltName = @alt_names
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth
[ alt_names ]
DNS.1 = localhost
DNS.2 = 127.0.0.1
DNS.3 = ::1
`

			if err := os.WriteFile("openssl.cnf", []byte(opensslConfig), 0644); err != nil {
				fmt.Printf("Error creating OpenSSL config: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ Created OpenSSL configuration")

			// Generate RSA private key with 2048 bits
			keyCmd := exec.Command("openssl", "genpkey",
				"-algorithm", "RSA",
				"-pkeyopt", "rsa_keygen_bits:2048",
				"-out", "key.pem")
			if err := keyCmd.Run(); err != nil {
				fmt.Printf("Error generating private key: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ Generated private key")

			// Set restrictive permissions on the private key
			if err := os.Chmod("key.pem", 0600); err != nil {
				fmt.Printf("Error setting key permissions: %v\n", err)
				os.Exit(1)
			}

			// Generate CSR using our config
			csrCmd := exec.Command("openssl", "req",
				"-new",
				"-key", "key.pem",
				"-out", "cert.csr",
				"-config", "openssl.cnf")
			if err := csrCmd.Run(); err != nil {
				fmt.Printf("Error generating CSR: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ Generated certificate signing request")

			// Generate self-signed certificate with proper extensions
			certCmd := exec.Command("openssl", "x509",
				"-req",
				"-days", "365",
				"-in", "cert.csr",
				"-signkey", "key.pem",
				"-out", "cert.pem",
				"-extensions", "req_ext",
				"-extfile", "openssl.cnf")
			if err := certCmd.Run(); err != nil {
				fmt.Printf("Error generating certificate: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ Generated self-signed certificate")

			// Clean up temporary files
			os.Remove("cert.csr")
			os.Remove("openssl.cnf")
			fmt.Println("✓ Cleaned up temporary files")
		}

		// Create .env template if it doesn't exist
		if _, err := os.Stat(".env"); os.IsNotExist(err) {
			envContent := `# OpenAI API Key (Required)
OPENAI_API_KEY=your_key_here

# Server Configuration
PORT=8080
CERT_FILE=cert.pem
KEY_FILE=key.pem

# Development Settings
GIN_MODE=debug
ALLOW_INSECURE=true  # Allows server to continue on TLS handshake errors
`
			if err := os.WriteFile(".env", []byte(envContent), 0644); err != nil {
				fmt.Printf("Error creating .env template: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("✓ Created .env template file")
		}

		fmt.Println("\n✨ Initialization complete!")
		fmt.Println("\nNext steps:")
		fmt.Println("1. Edit .env file and set your OpenAI API key:")
		fmt.Println("   OPENAI_API_KEY=your_key_here")
		fmt.Println("\n2. Start the server:")
		fmt.Println("   convinceme serve")
		fmt.Println("\n3. Configure Chrome:")
		fmt.Println("   a. Visit chrome://flags/#allow-insecure-localhost")
		fmt.Println("   b. Enable 'Allow invalid certificates for resources loaded from localhost'")
		fmt.Println("   c. Restart Chrome")
		fmt.Println("\n4. Access the application:")
		fmt.Println("   Open https://localhost:8080")
		fmt.Println("   (Accept the self-signed certificate warning if shown)")
		fmt.Println("\nNote: The server will now continue running even if TLS handshake errors occur.")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
