package goawshelpers

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ssm"
)

const defaultRegion = "eu-north-1"

// SSMConfiguration provides an easy way to access parameters from AWS Parameter Store
type SSMConfiguration struct {
	client       *ssm.SSM
	env          string
	keyDelimitor string
}

// SSMConfigurationInit helps to initialize SSMConfiguration using a variety of keys
type SSMConfigurationInit struct {
	env                string
	keyDelimitor       string
	awsAccessKey       string
	awsSecretAccessKey string
	useEnvParams       bool
	region             string
}

// NewSSMConfiguration creates a new instance of SSMConfiguration based on the passed in parameters
func NewSSMConfiguration(config SSMConfigurationInit) (*SSMConfiguration, error) {
	var creds *credentials.Credentials
	region := config.region

	if config.region == "" {
		region = defaultRegion
	}

	if !config.useEnvParams {
		if config.awsAccessKey == "" && config.awsSecretAccessKey == "" {
			return nil, fmt.Errorf("no awsAccessKey and/or awsSecretAccessKey provided")
		}

		creds = credentials.NewStaticCredentials(config.awsAccessKey, config.awsSecretAccessKey, "")
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

	return &SSMConfiguration{
		client: ssm.New(session, aws.NewConfig().WithRegion(region)),
		env:    config.env,
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

func convertKeynameToPath(key, env, delimiter string) string {
	return strings.ToLower(fmt.Sprintf("/%s/%s", env, strings.ReplaceAll(key, delimiter, "/")))
}

func convertPathToKeyname(path, env, delimiter string) string {
	key := strings.Replace(path, fmt.Sprintf("/%s/", env), "", 1)
	key = strings.ReplaceAll(key, "/", delimiter)
	return strings.ToLower(key)
}
