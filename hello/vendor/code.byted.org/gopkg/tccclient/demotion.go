package tccclient

import (
	"context"
	"fmt"
	"strconv"
)

type DemotionClient struct {
	*ClientV2
}

func NewDemotionClient(serviceName string, config *ConfigV2) (*DemotionClient, error) {
	clientV2, err := NewClientV2(serviceName, config)
	if err != nil {
		return nil, err
	}
	client := &DemotionClient{clientV2}
	return client, nil
}

// GetInt parse value to int
func (d *DemotionClient) GetInt(ctx context.Context, key string) (int, error) {
	value, err := d.Get(ctx, key)
	if err != nil {
		return 0, err
	}
	ret, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("GetInt Error: Key = %s; value = %s is not int", key, value)
	}
	return ret, nil
}

// GetBool parse value to bool:
//     if value=="0" return false;
//     if value=="1" return true;
//     if value!="0" && value!="1" return error;
func (d *DemotionClient) GetBool(ctx context.Context, key string) (bool, error) {
	value, err := d.Get(ctx, key)
	if err != nil {
		return false, err
	}
	switch value {
	case "0":
		return false, nil
	case "1":
		return true, nil
	default:
		return false, fmt.Errorf("GetBool Error: Key = %s; value = %s is not bool", key, value)
	}
}
