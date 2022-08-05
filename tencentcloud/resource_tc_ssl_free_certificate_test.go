package tencentcloud

import (
	"context"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	ssl2 "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ssl/v20191205"
	"github.com/tencentcloudstack/terraform-provider-tencentcloud/tencentcloud/internal/helper"
)

func init() {
	// go test -v ./tencentcloud -sweep=ap-guangzhou -sweep-run=tencentcloud_ssl_free_certificate
	resource.AddTestSweepers("tencentcloud_ssl_free_certificate", &resource.Sweeper{
		Name: "tencentcloud_ssl_free_certificate",
		F: func(r string) error {
			logId := getLogId(contextNil)
			ctx := context.WithValue(context.TODO(), logIdKey, logId)
			cli, _ := sharedClientForRegion(r)
			client := cli.(*TencentCloudClient).apiV3Conn
			service := SSLService{client}

			request := ssl2.NewDescribeCertificatesRequest()
			request.SearchKey = helper.String("my_free_cert")
			certs, err := service.DescribeCertificates(ctx, request)
			if err != nil {
				return err
			}

			for i := range certs {
				cert := certs[i]
				name := cert.Alias
				created, err := time.Parse("2006-01-02 15:04:05", *cert.InsertTime)
				if err != nil {
					created = time.Time{}
				}

				if isResourcePersist(*name, &created) {
					continue
				}
				request := ssl2.NewDeleteCertificateRequest()
				request.CertificateId = cert.CertificateId

				_, err = service.DeleteCertificate(ctx, request)
				if err != nil {
					continue
				}
			}

			return nil
		},
	})
}

func TestAccTencentCloudNeedFixSSLFreeCertificate(t *testing.T) {
	t.Parallel()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccSSLFreeCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config:  testAccSSLFreeCertificateBasic,
				Destroy: false,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckSslCertificateExists("tencentcloud_ssl_free_certificate.foo"),
					resource.TestCheckResourceAttr("tencentcloud_ssl_free_certificate.foo", "alias", "my_free_cert"),
					resource.TestCheckResourceAttrSet("tencentcloud_ssl_free_certificate.foo", "domain"),
				),
			},
		},
	})
}

func testAccSSLFreeCertificateDestroy(s *terraform.State) error {
	return nil
}

const testAccSSLFreeCertificateBasic = `
resource "tencentcloud_ssl_free_certificate" "foo" {
	dv_auth_method = "DNS_AUTO"
	domain = "example.com"
    package_type = "2"
	contact_email = "foo@example.com"
	contact_phone = "12345678901"
	validity_period = 12
	csr_encrypt_algo = "RSA"
	csr_key_parameter = "2048"
	csr_key_password = "xxxxxxxx"
	alias = "my_free_cert"
}
`
