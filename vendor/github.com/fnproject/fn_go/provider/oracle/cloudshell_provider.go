package oracle

import (
	"fmt"
	"os"

	"github.com/fnproject/fn_go/provider"
	oci "github.com/oracle/oci-go-sdk/common"
	"github.com/oracle/oci-go-sdk/common/auth"
)

const (
	OciRegion              = "region"
	OciDelegationTokenFile = "delegation_token_file"
)

// Holds the three required config values in a CS env.
type CloudShellConfig struct {
	tenancyID           string
	region              string
	delegationTokenFile string
}

// Creates a new "oracle-cs" provider instance for use when Fn is deployed in an OCI CloudShell environment.
func NewCSProvider(configSource provider.ConfigSource, passphraseSource provider.PassPhraseSource) (provider.Provider, error) {

	var csConfig *CloudShellConfig
	var err error

	// Derive oracle.profile from context or environment
	oraProfile := configSource.GetString(CfgProfile)
	envOraProfile := os.Getenv(OCI_CLI_PROFILE_ENV_VAR)
	if envOraProfile != "" {
		oraProfile = envOraProfile
	}
	// If the oracle.profile in env or context isn't DEFAULT then derive config from OCI profile
	if oraProfile != "" {
		csConfig, err = loadCSOracleConfig(oraProfile, passphraseSource)
		if err != nil {
			return nil, err
		}
	} else {
		csConfig = &CloudShellConfig{tenancyID: "", region: "", delegationTokenFile: ""}
	}

	// Now we read config from environment to either override base config, or instead of if config existed.
	region := os.Getenv(OCI_CLI_REGION_ENV_VAR)
	if region != "" {
		csConfig.region = region
	}
	if csConfig.region == "" {
		return nil, fmt.Errorf("Could not derive region from eiher config or environment.")
	}

	tenancyID := os.Getenv(OCI_CLI_TENANCY_ENV_VAR)
	if tenancyID != "" {
		csConfig.tenancyID = tenancyID
	}
	if csConfig.tenancyID == "" {
		return nil, fmt.Errorf("Could not derive tenancy ID from eiher config or environment.")
	}

	delegationTokenFile := os.Getenv(OCI_CLI_DELEGATION_TOKEN_FILE_ENV_VAR)
	if delegationTokenFile != "" {
		csConfig.delegationTokenFile = delegationTokenFile
	}
	if csConfig.delegationTokenFile == "" {
		return nil, fmt.Errorf("Could not derive delegation token filepath from eiher config or environment.")
	}

	// If we have an explicit api-url configured then use that, otherwise compute the url from the standard
	// production url form and the configured region from environment.
	cfgApiUrl := configSource.GetString(provider.CfgFnAPIURL)
	if cfgApiUrl == "" {
		cfgApiUrl = fmt.Sprintf(FunctionsAPIURLTmpl, csConfig.region)
	}
	apiUrl, err := provider.CanonicalFnAPIUrl(cfgApiUrl)
	if err != nil {
		return nil, err
	}
	//os.Stdout.WriteString("apiUrl:" + apiUrl.String())

	// If the compartment ID wasn't specified in the context, we default to the root compartment by using
	// the tenancy ID.
	compartmentID := configSource.GetString(CfgCompartmentID)
	if compartmentID == "" {
		compartmentID = csConfig.tenancyID
	}

	oboToken := ""
	provider, err := auth.InstancePrincipalConfigurationProvider()
	if err != nil {
		return nil, err
	}

	client, err := oci.NewClientWithOboToken(provider, oboToken)
	if err != nil {
		return nil, err
	}

	return &OracleProvider{
		FnApiUrl:      apiUrl,
		Signer:        client.Signer,
		Interceptor:   client.Interceptor,
		DisableCerts:  configSource.GetBool(CfgDisableCerts),
		CompartmentID: compartmentID,
	}, nil
}

func loadCSOracleConfig(profileName string, passphrase provider.PassPhraseSource) (*CloudShellConfig, error) {
	var err error
	var cf oci.ConfigurationProvider

	path := os.Getenv(OCI_CLI_CONFIG_FILE_ENV_VAR)
	if _, err := os.Stat(path); err == nil {
		cf, err = oci.ConfigurationProviderFromFileWithProfile(path, profileName, "")
		if err != nil {
			return nil, err
		}
	}

	region, err := cf.Region()
	if err != nil {
		return nil, err
	}

	tenancyOCID, err := cf.TenancyOCID()
	if err != nil {
		return nil, err
	}

	delegationTokenFile := "/etc/oci/delegation_token"
	//	delegationTokenFile, err := cf.DelegationTokenFile() // Not yet supported
	if err != nil {
		return nil, err
	}

	return &CloudShellConfig{tenancyID: tenancyOCID, region: region, delegationTokenFile: delegationTokenFile}, nil
}
