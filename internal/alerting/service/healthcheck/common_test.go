package healthcheck

import (
	"fmt"
	"math"
	"testing"
)

func TestCommon(t *testing.T) {
	baseTh := 97.0

	newThreshold := baseTh - math.Ceil(baseTh*0.01)
	fmt.Println(newThreshold)
}
