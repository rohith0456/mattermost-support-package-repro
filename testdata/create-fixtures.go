//go:build ignore
// +build ignore

// create-fixtures.go creates test ZIP fixtures for the testdata directory.
// Run: go run testdata/create-fixtures.go
package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if err := createMinimalFixture(); err != nil {
		fmt.Fprintf(os.Stderr, "minimal fixture: %v\n", err)
		os.Exit(1)
	}
	if err := createEnterpriseFixture(); err != nil {
		fmt.Fprintf(os.Stderr, "enterprise fixture: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Fixtures created successfully.")
}

func createMinimalFixture() error {
	path := filepath.Join("testdata", "support-packages", "minimal.zip")
	w, err := os.Create(path)
	if err != nil {
		return err
	}
	defer w.Close()

	zw := zip.NewWriter(w)
	defer zw.Close()

	config := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"Version": "8.1.0",
			"SiteURL": "http://localhost:8065",
		},
		"SqlSettings": map[string]interface{}{
			"DriverName": "postgres",
			"DataSource": "postgres://mmuser:password@localhost:5432/mattermost",
		},
		"ClusterSettings": map[string]interface{}{
			"Enable": false,
		},
		"FileSettings": map[string]interface{}{
			"DriverName": "local",
		},
		"PluginSettings": map[string]interface{}{
			"PluginStates": map[string]interface{}{
				"playbooks": map[string]interface{}{"Enable": true},
			},
		},
	}

	return addJSONFile(zw, "config.json", config)
}

func createEnterpriseFixture() error {
	path := filepath.Join("testdata", "support-packages", "enterprise.zip")
	w, err := os.Create(path)
	if err != nil {
		return err
	}
	defer w.Close()

	zw := zip.NewWriter(w)
	defer zw.Close()

	config := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"Version": "9.3.0",
			"SiteURL": "https://mattermost.company.internal",
			"TrustedProxyIPHeader": []string{"X-Forwarded-For"},
		},
		"SqlSettings": map[string]interface{}{
			"DriverName":         "postgres",
			"DataSource":         "postgres://mmuser:enterprise_pass@db:5432/mattermost",
			"DataSourceReplicas": []string{"postgres://mmuser:enterprise_pass@replica:5432/mattermost"},
		},
		"ClusterSettings": map[string]interface{}{
			"Enable":      true,
			"ClusterName": "prod-cluster",
		},
		"FileSettings": map[string]interface{}{
			"DriverName":              "amazons3",
			"AmazonS3Bucket":          "company-mattermost",
			"AmazonS3AccessKeyId":     "AKIAIOSFODNN7EXAMPLE",
			"AmazonS3SecretAccessKey": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
		"LdapSettings": map[string]interface{}{
			"Enable":       true,
			"LdapServer":   "ldap.internal.company.com",
			"BindPassword": "ldap_bind_secret",
		},
		"PluginSettings": map[string]interface{}{
			"PluginStates": map[string]interface{}{
				"playbooks":                {"Enable": true},
				"com.mattermost.calls":     {"Enable": true},
				"com.company.custom":       {"Enable": true},
			},
		},
		"LicenseSettings": map[string]interface{}{
			"ActiveBlobHash": "abc123licenseblob",
		},
	}

	if err := addJSONFile(zw, "config.json", config); err != nil {
		return err
	}

	clusterInfo := map[string]interface{}{
		"Nodes": []map[string]interface{}{
			{"id": "node1", "version": "9.3.0", "hostname": "mm1.internal"},
			{"id": "node2", "version": "9.3.0", "hostname": "mm2.internal"},
		},
	}
	return addJSONFile(zw, "cluster_info.json", clusterInfo)
}

func addJSONFile(zw *zip.Writer, name string, data interface{}) error {
	f, err := zw.Create(name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
