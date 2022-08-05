package main

import (
	"fmt"

	external_provider "github.com/tencentcloudstack/terraform-provider-tencentcloud/external"
)

func main() {
	fmt.Println(external_provider.Provider().DataSourcesMap)
}
