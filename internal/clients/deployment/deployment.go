package deployment

import (
	"context"

	"github.com/carlmjohnson/requests"
	"github.com/ghodss/yaml"
)

func Get(serviceUrl, deploymentId string) ([]byte, error) {
	tmp := map[string]any{}

	err := requests.
		URL(serviceUrl).Path(deploymentId).
		ToJSON(&tmp).
		CheckStatus(200).
		Fetch(context.Background())
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(tmp)
}
