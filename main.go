package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/yaml"
	"strings"
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
	err := kustomizeClient.RunBuild(os.Stdout)
	if err != nil {
		log.Fatal(err)
	}
}

// Options contain the options for running a build
type Options struct {
	kustomizationPath string
	outputPath        string
}

// NewOptions creates a Options object
func NewOptions(p, o string) *Options {
	return &Options{
		kustomizationPath: p,
		outputPath:        o,
	}
}

func (o *Options) RunBuild(out io.Writer) error {
	fSys := filesys.MakeFsOnDisk()
	options := &krusty.Options{
		DoLegacyResourceSort: false,
		LoadRestrictions:     types.LoadRestrictionsNone,
		DoPrune:              false,
		PluginConfig:         konfig.DisabledPluginConfig(),
	}
	k := krusty.MakeKustomizer(fSys, options)
	m, err := k.Run(o.kustomizationPath)
	if err != nil {
		return err
	}
	return o.emitResources(out, fSys, m)
}

func (o *Options) emitResources(out io.Writer, fSys filesys.FileSystem, m resmap.ResMap) error {
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

func writeIndividualFiles(
	fSys filesys.FileSystem, folderPath string, m resmap.ResMap) error {
	byNamespace := m.GroupedByCurrentNamespace()
	for namespace, resList := range byNamespace {
		for _, res := range resList {
			fName := fileName(res)
			if len(byNamespace) > 1 {
				fName = strings.ToLower(namespace) + "_" + fName
			}
			err := writeFile(fSys, folderPath, fName, res)
			if err != nil {
				return err
			}
		}
	}
	for _, res := range m.NonNamespaceable() {
		err := writeFile(fSys, folderPath, fileName(res), res)
		if err != nil {
			return err
		}
	}
	return nil
}

func fileName(res *resource.Resource) string {
	return res.GetKind() + "_" + res.GetName() + ".yaml"
}

func writeFile(fSys filesys.FileSystem, path, fName string, res *resource.Resource) error {
	out, err := yaml.Marshal(res.Map())
	if err != nil {
		return err
	}
	return fSys.WriteFile(filepath.Join(path, fName), out)
}
