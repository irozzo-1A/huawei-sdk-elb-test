package main

import (
	"os"

	"flag"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/huaweicloud/golangsdk"
	huaweisdk "github.com/huaweicloud/golangsdk/openstack"
	"github.com/huaweicloud/golangsdk/openstack/networking/v2/extensions/elbaas/listeners"
	"github.com/huaweicloud/golangsdk/openstack/networking/v2/extensions/elbaas/loadbalancer_elbs"

	"github.com/google/uuid"
	"k8s.io/klog"
)

var (
	identityEndpoint string
	projectName      string
	projectID        string
	region           string
	accessKey        string
	secretKey        string
)

func init() {
	flag.Set("v", "10")
	flag.StringVar(&identityEndpoint, "identity-endpoint", "https://iam.eu-de.otc.t-systems.com/v3", "Identity endpoint")
	flag.StringVar(&projectName, "project", "", "Project name")
	flag.StringVar(&projectID, "project-id", "", "Project ID")
	flag.StringVar(&region, "region", "eu-de", "Region")
	flag.StringVar(&accessKey, "access-key", "", "Access key")
	flag.StringVar(&secretKey, "secret-key", "", "Secret key")
}

func main() {
	flag.Parse()
	if projectName == "" && projectID == "" {
		klog.Fatal("At least one between project name and project ID should be given")
		os.Exit(1)
	}
	if accessKey == "" || secretKey == "" {
		klog.Fatal("access key and secret key should be given")
		os.Exit(1)
	}
	akskOpts := golangsdk.AKSKAuthOptions{
		IdentityEndpoint: identityEndpoint,
		ProjectName:      projectName,
		ProjectId:        projectID,
		Region:           region,
		AccessKey:        accessKey,
		SecretKey:        secretKey,
	}

	providerClient, err := huaweisdk.NewClient(akskOpts.GetIdentityEndpoint())
	if err != nil {
		klog.Fatalf("provider client creation failed with error: %v", err)
		os.Exit(1)
	}

	providerClient.HTTPClient = HTTPClientConfig{}.New()

	err = huaweisdk.Authenticate(providerClient, akskOpts)
	if err != nil {
		klog.Fatalf("authentication failed with error: %v", err)
		os.Exit(1)
	}

	client, _ := huaweisdk.NewELBV1(providerClient, golangsdk.EndpointOpts{
		Region:       "eu-de",
		Availability: golangsdk.AvailabilityPublic,
	})

	/*adminStateUp := true
	co := loadbalancer_elbs.CreateOpts{
		Name:         "elb",
		Description:  "test elb",
		VpcID:        "af1f447d-db14-4620-8874-0e99be52b360",
		AdminStateUp: &adminStateUp,
		Bandwidth:    5,
		Type:         "External",
	}
	job, err := loadbalancer_elbs.Create(client, co).ExtractJobResponse()
	if err != nil {
		klog.Errorf("error occurred while creating LB", err)
		os.Exit(1)
	}
	if err := golangsdk.WaitForJobSuccess(client, job.URI, 30); err != nil {
		klog.Errorf("error while waiting for LB creation", err)
		os.Exit(1)
	}*/
	l, err := listeners.List(client, listeners.ListOpts{}).AllPages()
	if err != nil {
		klog.Errorf("error occurred while getting all pages: %v", err)
		os.Exit(1)
	}
	ls, err := listeners.ExtractListeners(l)
	if err != nil {
		klog.Errorf("error occurred while extracting listeners: %v", err)
		os.Exit(1)
	}
	klog.Infof("Listeners: %+v", ls)
	lb, err := loadbalancer_elbs.List(client, loadbalancer_elbs.ListOpts{}).AllPages()
	if err != nil {
		klog.Errorf("error occurred while getting all pages: %v", err)
		os.Exit(1)
	}
	lbs, err := loadbalancer_elbs.ExtractLoadBalancers(lb)
	if err != nil {
		klog.Errorf("error occurred while extracting lbs: %v", err)
		os.Exit(1)
	}
	klog.Infof("ELBs: %+v", lbs)
}

const defaultClientTimeout = 15 * time.Second

type HTTPClientConfig struct {
	// LogPrefix is pre-pended to request/response logs
	LogPrefix string
	// Global timeout used by the client
	Timeout time.Duration
}

// New return a custom HTTP client that allows for logging
// HTTP request and response information.
func (c HTTPClientConfig) New() http.Client {
	timeout := c.Timeout
	// Enforce a global timeout
	if timeout <= 0 {
		timeout = defaultClientTimeout
	}
	return http.Client{
		Transport: &LogRoundTripper{
			logPrefix: c.LogPrefix,
			rt:        http.DefaultTransport,
		},
		Timeout: timeout,
	}
}

// LogRoundTripper is used to log information about requests and responses that
// may be useful for debugging purposes.
// Note that setting log level >5 results in full dumps of requests and
// responses, including sensitive invormation (e.g. Authorization header).
type LogRoundTripper struct {
	logPrefix string
	rt        http.RoundTripper
}

func (lrt *LogRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	var log []byte
	var err error
	// Generate unique ID to correlate requests and responses
	id := uuid.New()
	log, err = httputil.DumpRequest(request, true)
	if err != nil {
		klog.Warningf("Error occurred while dumping request: %v", err)
	}
	klog.Infof("%s request sent [%s]: %s\n", lrt.logPrefix, id.String(), string(log))

	response, err := lrt.rt.RoundTrip(request)
	if response == nil {
		return nil, err
	}

	log, err = httputil.DumpResponse(response, true)
	if err != nil {
		klog.Warningf("Error occurred while dumping response: %v", err)
	}
	klog.Infof("%s request received [%s]: %s\n", lrt.logPrefix, id.String(), string(log))

	return response, nil
}

// Return value if nonempty, def otherwise.
func valueOrDefault(value, def string) string {
	if value != "" {
		return value
	}
	return def
}
