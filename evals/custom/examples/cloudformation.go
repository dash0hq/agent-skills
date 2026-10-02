package examples

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidateCloudFormation checks the small set of structural invariants that
// fenced examples can verify without pretending to replace AWS service-side
// schema validation.
func ValidateCloudFormation(content string) error {
	var root map[string]any
	if err := yaml.Unmarshal([]byte(content), &root); err != nil {
		return fmt.Errorf("cloudformation: not valid YAML: %w", err)
	}

	resourcesRaw, ok := root["Resources"]
	if !ok {
		return fmt.Errorf("cloudformation: missing Resources")
	}
	resources, ok := resourcesRaw.(map[string]any)
	if !ok || len(resources) == 0 {
		return fmt.Errorf("cloudformation: Resources must be a non-empty mapping")
	}

	return validateCloudFormationResources(resources)
}

func validateCloudFormationResources(resources map[string]any) error {
	for logicalID, raw := range resources {
		if strings.HasPrefix(logicalID, "Fn::ForEach::") {
			if err := validateCloudFormationForEach(logicalID, raw); err != nil {
				return err
			}
			continue
		}

		resource, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("cloudformation: resource %q must be a mapping", logicalID)
		}
		resourceType, ok := resource["Type"].(string)
		if !ok || resourceType == "" {
			return fmt.Errorf("cloudformation: resource %q has invalid Type", logicalID)
		}
		if resourceType != "AWS::Lambda::Function" && resourceType != "AWS::Serverless::Function" {
			continue
		}

		propertiesRaw, ok := resource["Properties"]
		if !ok {
			return fmt.Errorf("cloudformation: Lambda resource %q is missing Properties", logicalID)
		}
		properties, ok := propertiesRaw.(map[string]any)
		if !ok {
			return fmt.Errorf("cloudformation: Lambda resource %q Properties must be a mapping", logicalID)
		}

		packageTypeRaw, hasPackageType := properties["PackageType"]
		packageType, packageTypeKnown := packageTypeRaw.(string)
		if !hasPackageType {
			packageType = "Zip"
			packageTypeKnown = true
		}
		if packageTypeKnown && packageType != "Zip" && packageType != "Image" {
			return fmt.Errorf("cloudformation: Lambda resource %q has invalid literal PackageType %q", logicalID, packageType)
		}
		if _, hasLayers := properties["Layers"]; packageTypeKnown && packageType == "Image" && hasLayers {
			return fmt.Errorf("cloudformation: Lambda resource %q uses PackageType Image and cannot use Layers", logicalID)
		}
	}

	return nil
}

func validateCloudFormationForEach(logicalID string, raw any) error {
	parts, ok := raw.([]any)
	if !ok || len(parts) != 3 {
		return fmt.Errorf("cloudformation: %q must contain an identifier, collection, and output fragment", logicalID)
	}
	if identifier, ok := parts[0].(string); !ok || identifier == "" {
		return fmt.Errorf("cloudformation: %q has an invalid identifier", logicalID)
	}
	fragment, ok := parts[2].(map[string]any)
	if !ok || len(fragment) == 0 {
		return fmt.Errorf("cloudformation: %q output fragment must be a non-empty mapping", logicalID)
	}
	return validateCloudFormationResources(fragment)
}
