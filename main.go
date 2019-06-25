package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/v3/k8sdeps/kunstruct"
	transformerDeps "sigs.k8s.io/kustomize/v3/k8sdeps/transformer"
	"sigs.k8s.io/kustomize/v3/k8sdeps/validator"
	"sigs.k8s.io/kustomize/v3/pkg/fs"
	"sigs.k8s.io/kustomize/v3/pkg/ifc"
	"sigs.k8s.io/kustomize/v3/pkg/ifc/transformer"
	"sigs.k8s.io/kustomize/v3/pkg/loader"
	"sigs.k8s.io/kustomize/v3/pkg/plugins"
	"sigs.k8s.io/kustomize/v3/pkg/resmap"
	"sigs.k8s.io/kustomize/v3/pkg/resource"
	"sigs.k8s.io/kustomize/v3/pkg/target"
	"sigs.k8s.io/yaml"
)

func main() {
	kustomizationFilePath := "examples/botkube/overlay/default"
	kustomizationOutput := "output"
	if _, err := os.Stat(kustomizationOutput); os.IsNotExist(err) {
		err = os.Mkdir(kustomizationOutput, 0744)
		if err != nil {
			log.Panic(err)
		}
	}

	// New Kustomization
	kustomizeClient := NewOptions(kustomizationFilePath, kustomizationOutput)

	stdOut := os.Stdout
	fSys := fs.MakeRealFS()
	uf := kunstruct.NewKunstructuredFactoryImpl()
	rf := resmap.NewFactory(resource.NewFactory(uf))
	v := validator.NewKustValidator()
	ptf := transformerDeps.NewFactoryImpl()

	pluginConfig := plugins.DefaultPluginConfig()
	pl := plugins.NewLoader(pluginConfig, rf)

	err := kustomizeClient.RunBuild(stdOut, v, fSys, rf,  ptf, pl)
	if err != nil {
		log.Fatal(err)
	}
}

type Options struct {
	kustomizationPath string
	outputPath        string
	loadRestrictor    loader.LoadRestrictorFunc
}

// NewOptions creates a Options object
func NewOptions(inputPath, outputPath string) *Options {
	return &Options{
		kustomizationPath: inputPath,
		outputPath:        outputPath,
		loadRestrictor:    loader.RestrictionRootOnly,
	}
}

func (o *Options) RunBuild(out io.Writer, v ifc.Validator, fSys fs.FileSystem, rf *resmap.Factory, ptf transformer.Factory, pl *plugins.Loader) error {
	ldr, err := loader.NewLoader(o.loadRestrictor, v, o.kustomizationPath, fSys)
	if err != nil {
		return err
	}
	defer ldr.Cleanup()
	kt, err := target.NewKustTarget(ldr, rf, ptf, pl)
	if err != nil {
		return err
	}
	m, err := kt.MakeCustomizedResMap()
	if err != nil {
		return err
	}

	return o.emitResources(out, fSys, m)
}

func (o *Options) emitResources(out io.Writer, fSys fs.FileSystem, m resmap.ResMap) error {
	if o.outputPath != "" && fSys.IsDir(o.outputPath) {
		return writeIndividualFiles(fSys, o.outputPath, m)
	}
	res, err := m.AsYaml()
	if err != nil {
		return err
	}
	if o.outputPath != "" {
		return fSys.WriteFile(o.outputPath, res)
	}
	_, err = out.Write(res)
	return err
}

func writeIndividualFiles(fSys fs.FileSystem, folderPath string, m resmap.ResMap) error {
	for _, res := range m.Resources() {
		filename := filepath.Join(
			folderPath,
			fmt.Sprintf(
				"%s_%s.yaml",
				res.GetKind(),
				res.GetName(),
			),
		)
		out, err := yaml.Marshal(res.Map())
		if err != nil {
			return err
		}
		err = fSys.WriteFile(filename, out)
		if err != nil {
			return err
		}
	}
	return nil
}
