package examples

import (
	"strings"
	"testing"
)

func TestCloudFormationAnnotationAndHeuristic(t *testing.T) {
	content := strings.Join([]string{
		"<!-- eval:cloudformation -->",
		"```yaml",
		"Resources:",
		"  Fn:",
		"    Type: AWS::Lambda::Function",
		"    Properties:",
		"      Runtime: nodejs22.x",
		"      Handler: index.handler",
		"```",
	}, "\n")
	blocks, err := extract("inline.md", content)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(blocks) != 1 || blocks[0].Annotation != AnnotationCloudFormation {
		t.Fatalf("cloudformation annotation not recognized: %+v", blocks)
	}
	if got := Classify(blocks[0].Documents()[0]); got != CategoryCloudFormation {
		t.Fatalf("annotated template classified as %q, want %q", got, CategoryCloudFormation)
	}

	if got := classifyContent(t, "yaml", `AWSTemplateFormatVersion: '2010-09-09'
Resources:
  Fn:
    Type: AWS::Lambda::Function
    Properties:
      Runtime: nodejs22.x
      Handler: index.handler
`, AnnotationNone); got != CategoryCloudFormation {
		t.Fatalf("CloudFormation heuristic = %q, want %q", got, CategoryCloudFormation)
	}
}

func TestValidateCloudFormationLambdaLayers(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "zip function with literal layers",
			content: `AWSTemplateFormatVersion: '2010-09-09'
Resources:
  Fn:
    Type: AWS::Lambda::Function
    Properties:
      Runtime: nodejs22.x
      Handler: index.handler
      Layers:
        - arn:aws:lambda:eu-west-1:123456789012:layer:otel:1
  Extension:
    Type: Vendor::Observability::Extension
`,
		},
		{
			name: "SAM function with intrinsic layers",
			content: `Resources:
  Fn:
    Type: AWS::Serverless::Function
    Properties:
      Runtime: nodejs22.x
      Handler: index.handler
      Layers:
        Ref: OTelLayers
`,
		},
		{
			name: "image function without layers",
			content: `Resources:
  Fn:
    Type: AWS::Lambda::Function
    Properties:
      PackageType: Image
      Code:
        ImageUri: example.invalid/repo/image:TEST-1
`,
		},
		{
			name: "intrinsic package type remains unknown",
			content: `Resources:
  Fn:
    Type: AWS::Lambda::Function
    Properties:
      PackageType:
        Ref: FunctionPackageType
      Layers:
        Ref: OTelLayers
`,
		},
		{
			name: "LanguageExtensions loop in Resources",
			content: `Transform: AWS::LanguageExtensions
Resources:
  Fn::ForEach::Functions:
    - FunctionName
    - [Checkout, Reconcile]
    - ${FunctionName}Function:
        Type: AWS::Serverless::Function
        Properties:
          Runtime: nodejs22.x
          Handler: index.handler
`,
		},
		{
			name:    "missing Resources",
			content: "AWSTemplateFormatVersion: '2010-09-09'\n",
			wantErr: "missing Resources",
		},
		{
			name: "resource is not a mapping",
			content: `Resources:
  Fn: invalid
`,
			wantErr: "must be a mapping",
		},
		{
			name: "Lambda properties are missing",
			content: `Resources:
  Fn:
    Type: AWS::Lambda::Function
`,
			wantErr: "missing Properties",
		},
		{
			name: "invalid literal package type",
			content: `Resources:
  Fn:
    Type: AWS::Lambda::Function
    Properties:
      PackageType: Archive
`,
			wantErr: "invalid literal PackageType",
		},
		{
			name: "image function with layers",
			content: `Resources:
  Fn:
    Type: AWS::Lambda::Function
    Properties:
      PackageType: Image
      Code:
        ImageUri: example.invalid/repo/image:TEST-1
      Layers:
        - arn:aws:lambda:eu-west-1:123456789012:layer:otel:1
`,
			wantErr: "cannot use Layers",
		},
		{
			name: "malformed ForEach",
			content: `Transform: AWS::LanguageExtensions
Resources:
  Fn::ForEach::Functions:
    - FunctionName
    - [Checkout, Reconcile]
`,
			wantErr: "must contain an identifier, collection, and output fragment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCloudFormation(tt.content)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("valid template rejected: %v", err)
			}
			if tt.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErr)) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDocumentCloudFormation(t *testing.T) {
	validator := &Validator{}
	block := &Block{
		File:       "inline.md",
		Line:       1,
		Tag:        "yaml",
		Annotation: AnnotationCloudFormation,
		Content: `Resources:
  Fn:
    Type: AWS::Lambda::Function
    Properties:
      Runtime: nodejs22.x
      Handler: index.handler
`,
	}
	results := validator.validateDocument(block.Documents()[0])
	if len(results) != 1 || results[0].Status != StatusValidated {
		t.Fatalf("CloudFormation document did not validate: %+v", results)
	}
}
