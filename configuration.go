package goawshelpers

import (
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const (
	defaultRegion       = "eu-north-1"
	defaultKeyDelimitor = "_"
)

// Configuration interface
// SSMConfiguration follows this interface
type Configuration interface {
	Create(key, value string) error
	Set(key, value string) error
	Delete(key string) error
	Get(key string) (string, error)
	GetEnvironment() (map[string]string, error)
}

// SSMConfiguration provides an easy way to access parameters from AWS Parameter Store
type SSMConfiguration struct {
	client       *ssm.SSM
	env          string
	keyDelimitor string
}

// SSMConfigurationInit helps to initialize SSMConfiguration using a variety of keys
type SSMConfigurationInit struct {
	Env                string
	KeyDelimitor       string
	AwsAccessKey       string
	AwsSecretAccessKey string
	UseEnvParams       bool
	Region             string
}

// EnvironmentConfiguration helps with managing environmental variables
type EnvironmentConfiguration struct {
	UseUpper bool
	Values   map[string]string
}

// BiConfiguration checks both SSM key store and env for variables (SSM first)
// If SSM configuration not provided it will act as regular EnvironmentConfiguration
type BiConfiguration struct {
	ssmConfiguration *SSMConfiguration
	envConfiguration *EnvironmentConfiguration
	values           map[string]string
}

// NewSSMConfiguration creates a new instance of SSMConfiguration based on the passed in parameters
func NewSSMConfiguration(config SSMConfigurationInit) (*SSMConfiguration, error) {
	var creds *credentials.Credentials
	region := config.Region

	if config.Region == "" {
		region = defaultRegion
	}

	if !config.UseEnvParams {
		if config.AwsAccessKey == "" && config.AwsSecretAccessKey == "" {
			return nil, fmt.Errorf("no awsAccessKey and/or awsSecretAccessKey provided")
		}

		creds = credentials.NewStaticCredentials(config.AwsAccessKey, config.AwsSecretAccessKey, "")
	} else {
		creds = credentials.NewEnvCredentials()
	}

	session, err := session.NewSession(&aws.Config{
		Region:      aws.String(region),
		Credentials: creds,
	})

	if err != nil {
		return nil, fmt.Errorf("error initializing aws session - %w", err)
	}

	if config.KeyDelimitor == "" {
		config.KeyDelimitor = defaultKeyDelimitor
	}

	return &SSMConfiguration{
		client:       ssm.New(session, aws.NewConfig().WithRegion(region)),
		env:          config.Env,
		keyDelimitor: config.KeyDelimitor,
	}, nil
}

// Create creates a new entry in AWS SSM Parameter Store. If the key already exists - an error is returned
func (c *SSMConfiguration) Create(key, value string) error {
	if err := c.put(key, value, false); err != nil {
		return fmt.Errorf("error creating a new entry - %w", err)
	}
	return nil
}

// Set creates or updates an entry in AWS SSM Parameter Store
func (c *SSMConfiguration) Set(key, value string) error {
	if err := c.put(key, value, true); err != nil {
		return fmt.Errorf("error setting an entry with key %s - %w", key, err)
	}
	return nil
}

// Delete deletes the remote key
func (c *SSMConfiguration) Delete(key string) error {
	_, err := c.client.DeleteParameter(&ssm.DeleteParameterInput{
		Name: aws.String(convertKeynameToPath(key, c.env, c.keyDelimitor)),
	})

	if err != nil {
		return fmt.Errorf("error deleting key %s - %w", key, err)
	}
	return nil
}

// Get returns a key from remote AWS SSM Parameter Store
func (c *SSMConfiguration) Get(key string) (string, error) {
	param, err := c.client.GetParameter(&ssm.GetParameterInput{
		Name: aws.String(convertKeynameToPath(key, c.env, c.keyDelimitor)),
	})

	if err != nil {
		return "", fmt.Errorf("error retrieving key %s - %w", key, err)
	}

	return *param.Parameter.Value, nil
}

// GetEnvironment returns all the keys existing inside the environment
// Environment is taken from the *SSMConfiguration struct
func (c *SSMConfiguration) GetEnvironment() (map[string]string, error) {
	values := make(map[string]string)

	err := c.client.GetParametersByPathPages(&ssm.GetParametersByPathInput{
		Path:      aws.String(fmt.Sprintf("/%s/", c.env)),
		Recursive: aws.Bool(true),
	}, func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {
		for _, param := range page.Parameters {
			key := convertPathToKeyname(*param.Name, c.env, c.keyDelimitor)
			values[key] = *param.Value
		}
		return !lastPage
	})

	if err != nil {
		return nil, fmt.Errorf("error retrieving parameters by environment - %w", err)
	}

	return values, nil
}

func (c *SSMConfiguration) put(key, value string, overwrite bool) error {
	_, err := c.client.PutParameter(&ssm.PutParameterInput{
		Name:      aws.String(convertKeynameToPath(key, c.env, c.keyDelimitor)),
		Value:     aws.String(value),
		Type:      aws.String("string"),
		Overwrite: aws.Bool(overwrite),
	})

	return err
}

// Get returns the key from environment
func (c *EnvironmentConfiguration) Get(key string) (string, error) {
	value := os.Getenv(key)

	if value != "" {
		c.Values[key] = value
	}

	if value == "" {
		return "", fmt.Errorf("no value with key %s", key)
	}

	return value, nil
}

// Set sets the environmental variable
func (c *EnvironmentConfiguration) Set(key, value string) error {
	err := os.Setenv(key, value)

	if err != nil {
		return fmt.Errorf("error setting environmental variable %s - %w", key, err)
	}

	c.Values[key] = value

	return nil
}

// GetEnvironment returns all previously used variables
func (c *EnvironmentConfiguration) GetEnvironment() (map[string]string, error) {
	return c.Values, nil
}

// Delete destroys the variable from the environment and "cache"
func (c *EnvironmentConfiguration) Delete(key string) error {
	err := os.Unsetenv(key)

	if err != nil {
		return fmt.Errorf("error unsetting environmental variable %s - %w", key, err)
	}
	delete(c.Values, key)

	return nil
}

// NewBiConfiguration returns a new instance of dual configuration
func NewBiConfiguration(env EnvironmentConfiguration, ssmConfig *SSMConfigurationInit) (*BiConfiguration, error) {
	config := &BiConfiguration{
		envConfiguration: &env,
	}

	if ssmConfig != nil {
		ssmConfiguration, err := NewSSMConfiguration(*ssmConfig)

		if err != nil {
			return nil, fmt.Errorf("error creating SSM configuration - %w", err)
		}
		config.ssmConfiguration = ssmConfiguration
	}

	return config, nil
}

// Get returns a value from configurations
// First it checkks the ssm (if applicable) and then the env
func (c *BiConfiguration) Get(key string) (string, error) {
	if val, ok := c.values[key]; ok {
		return val, nil
	}

	if c.ssmConfiguration != nil {
		val, err := c.ssmConfiguration.Get(key)

		if err == nil {
			return val, nil
		}
	}

	val, err := c.envConfiguration.Get(key)

	if err != nil {
		return "", err
	}

	return val, nil
}

// GetEnvironment returns all the variables from the ssm based on environment and also the loaded ones from local env
func (c *BiConfiguration) GetEnvironment() (map[string]string, error) {
	values, _ := c.envConfiguration.GetEnvironment()

	if c.ssmConfiguration != nil {
		sValues, err := c.ssmConfiguration.GetEnvironment()

		if err != nil {
			return nil, fmt.Errorf("error retrieving environment - %w", err)
		}

		for k, v := range sValues {
			values[k] = v
		}
	}

	return values, nil
}

// Delete deletes the keys from both configurations
func (c *BiConfiguration) Delete(key string) error {
	if c.ssmConfiguration != nil {
		c.ssmConfiguration.Delete(key)
	}
	c.envConfiguration.Delete(key)

	delete(c.values, key)
	delete(c.envConfiguration.Values, key)

	return nil
}

func convertKeynameToPath(key, env, delimiter string) string {
	return strings.ToLower(fmt.Sprintf("/%s/%s", env, strings.ReplaceAll(key, delimiter, "/")))
}

func convertPathToKeyname(path, env, delimiter string) string {
	key := strings.Replace(path, fmt.Sprintf("/%s/", env), "", 1)
	key = strings.ReplaceAll(key, "/", delimiter)
	return strings.ToLower(key)
}
