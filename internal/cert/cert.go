package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	CACommonName     = "OpenHijack_CA"
	DefaultDomain    = "api.openai.com"
	OpenRouterDomain = "openrouter.ai"
	caValidityYears  = 100
	srvValidityDays  = 365
	rsaKeyBits       = 2048
)

type CertManager struct {
	caDir string
}

func NewCertManager(dataDir string) *CertManager {
	return &CertManager{caDir: filepath.Join(dataDir, "ca")}
}

func (cm *CertManager) CACertFile() string  { return filepath.Join(cm.caDir, "ca.crt") }
func (cm *CertManager) CAKeyFile() string   { return filepath.Join(cm.caDir, "ca.key") }
func (cm *CertManager) SrvCertFile() string { return filepath.Join(cm.caDir, DefaultDomain+".crt") }
func (cm *CertManager) SrvKeyFile() string  { return filepath.Join(cm.caDir, DefaultDomain+".key") }
func (cm *CertManager) OpenRouterCertFile() string {
	return filepath.Join(cm.caDir, OpenRouterDomain+".crt")
}
func (cm *CertManager) OpenRouterKeyFile() string {
	return filepath.Join(cm.caDir, OpenRouterDomain+".key")
}
func (cm *CertManager) CADir() string { return cm.caDir }

func (cm *CertManager) ensureCADir() error {
	return os.MkdirAll(cm.caDir, 0755)
}

func (cm *CertManager) HasCA() bool {
	_, err1 := os.Stat(cm.CACertFile())
	_, err2 := os.Stat(cm.CAKeyFile())
	return err1 == nil && err2 == nil
}

func (cm *CertManager) HasServerCert() bool {
	_, err1 := os.Stat(cm.SrvCertFile())
	_, err2 := os.Stat(cm.SrvKeyFile())
	_, err3 := os.Stat(cm.OpenRouterCertFile())
	_, err4 := os.Stat(cm.OpenRouterKeyFile())
	return (err1 == nil && err2 == nil) || (err3 == nil && err4 == nil)
}

func (cm *CertManager) GenerateCA(logf func(string, ...interface{})) error {
	if err := cm.ensureCADir(); err != nil {
		return fmt.Errorf("创建 CA 目录失败: %w", err)
	}

	key, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return fmt.Errorf("生成 CA 私钥失败: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("生成序列号失败: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   CACommonName,
			Organization: []string{"OpenHijack"},
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(caValidityYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            3,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("创建 CA 证书失败: %w", err)
	}

	if err := writePEM(cm.CAKeyFile(), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key)); err != nil {
		return fmt.Errorf("写入 CA 私钥失败: %w", err)
	}
	logf("CA 私钥生成成功: %s", cm.CAKeyFile())

	if err := writePEM(cm.CACertFile(), "CERTIFICATE", certDER); err != nil {
		return fmt.Errorf("写入 CA 证书失败: %w", err)
	}
	logf("CA 证书生成成功: %s", cm.CACertFile())

	return nil
}

func (cm *CertManager) GenerateServerCert(logf func(string, ...interface{})) error {
	if !cm.HasCA() {
		return fmt.Errorf("CA 证书不存在，请先生成 CA")
	}

	caCertPEM, err := os.ReadFile(cm.CACertFile())
	if err != nil {
		return fmt.Errorf("读取 CA 证书失败: %w", err)
	}
	caKeyPEM, err := os.ReadFile(cm.CAKeyFile())
	if err != nil {
		return fmt.Errorf("读取 CA 私钥失败: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return fmt.Errorf("解析 CA 证书 PEM 失败")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析 CA 证书失败: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return fmt.Errorf("解析 CA 私钥 PEM 失败")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析 CA 私钥失败: %w", err)
	}

	srvKey, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return fmt.Errorf("生成服务器私钥失败: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("生成序列号失败: %w", err)
	}

	srvTmpl := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:            []string{"CN"},
			Province:           []string{"Zhejiang"},
			Locality:           []string{"Hangzhou"},
			Organization:       []string{"OpenAI"},
			OrganizationalUnit: []string{"OpenAI"},
			CommonName:         DefaultDomain,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(0, 0, srvValidityDays),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              []string{DefaultDomain},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	srvCertDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("签发服务器证书失败: %w", err)
	}

	if err := writePEM(cm.SrvKeyFile(), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(srvKey)); err != nil {
		return fmt.Errorf("写入服务器私钥失败: %w", err)
	}
	logf("服务器私钥生成成功: %s", cm.SrvKeyFile())

	if err := writePEM(cm.SrvCertFile(), "CERTIFICATE", srvCertDER); err != nil {
		return fmt.Errorf("写入服务器证书失败: %w", err)
	}
	logf("服务器证书生成成功: %s", cm.SrvCertFile())

	return nil
}

func (cm *CertManager) GenerateOpenRouterCert(logf func(string, ...interface{})) error {
	if !cm.HasCA() {
		return fmt.Errorf("CA 证书不存在，请先生成 CA")
	}

	caCertPEM, err := os.ReadFile(cm.CACertFile())
	if err != nil {
		return fmt.Errorf("读取 CA 证书失败: %w", err)
	}
	caKeyPEM, err := os.ReadFile(cm.CAKeyFile())
	if err != nil {
		return fmt.Errorf("读取 CA 私钥失败: %w", err)
	}

	caCertBlock, _ := pem.Decode(caCertPEM)
	if caCertBlock == nil {
		return fmt.Errorf("解析 CA 证书 PEM 失败")
	}
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析 CA 证书失败: %w", err)
	}

	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return fmt.Errorf("解析 CA 私钥 PEM 失败")
	}
	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return fmt.Errorf("解析 CA 私钥失败: %w", err)
	}

	srvKey, err := rsa.GenerateKey(rand.Reader, rsaKeyBits)
	if err != nil {
		return fmt.Errorf("生成服务器私钥失败: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("生成序列号失败: %w", err)
	}

	srvTmpl := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Country:            []string{"CN"},
			Province:           []string{"Zhejiang"},
			Locality:           []string{"Hangzhou"},
			Organization:       []string{"OpenRouter"},
			OrganizationalUnit: []string{"OpenRouter"},
			CommonName:         OpenRouterDomain,
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(0, 0, srvValidityDays),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
		DNSNames:              []string{OpenRouterDomain},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}

	srvCertDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("签发服务器证书失败: %w", err)
	}

	if err := writePEM(cm.OpenRouterKeyFile(), "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(srvKey)); err != nil {
		return fmt.Errorf("写入服务器私钥失败: %w", err)
	}
	logf("OpenRouter 私钥生成成功: %s", cm.OpenRouterKeyFile())

	if err := writePEM(cm.OpenRouterCertFile(), "CERTIFICATE", srvCertDER); err != nil {
		return fmt.Errorf("写入服务器证书失败: %w", err)
	}
	logf("OpenRouter 证书生成成功: %s", cm.OpenRouterCertFile())

	return nil
}

func (cm *CertManager) TLSCert() (tlsCertFile, tlsKeyFile string) {
	return cm.SrvCertFile(), cm.SrvKeyFile()
}

func (cm *CertManager) ExtraTLSCerts() map[string]string {
	return map[string]string{
		OpenRouterDomain: filepath.Join(cm.caDir, OpenRouterDomain),
	}
}

func (cm *CertManager) RemoveLocalArtifacts(logf func(string, ...interface{})) error {
	paths := []string{
		cm.SrvCertFile(),
		cm.SrvKeyFile(),
		cm.OpenRouterCertFile(),
		cm.OpenRouterKeyFile(),
		cm.CACertFile(),
		cm.CAKeyFile(),
	}

	var errs []error
	for _, path := range paths {
		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				logf("本地文件不存在，跳过移除: %s", path)
				continue
			}
			errs = append(errs, fmt.Errorf("移除 %s 失败: %w", path, err))
			continue
		}
		logf("已移除本地文件: %s", path)
	}

	if err := os.Remove(cm.CADir()); err != nil {
		if !os.IsNotExist(err) && !errors.Is(err, syscall.ENOTEMPTY) {
			errs = append(errs, fmt.Errorf("移除 CA 目录失败: %w", err))
		}
	} else {
		logf("已移除 CA 目录: %s", cm.CADir())
	}

	return errors.Join(errs...)
}

func writePEM(path, pemType string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return pem.Encode(f, &pem.Block{Type: pemType, Bytes: data})
}
