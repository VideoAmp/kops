/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"io"
	"fmt"
	"github.com/spf13/cobra"
	"k8s.io/kops/cmd/kops/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	k8sapi "k8s.io/kubernetes/pkg/api"
	"github.com/golang/glog"
	kopsapi "k8s.io/kops/pkg/apis/kops"
	"k8s.io/kops/pkg/apis/kops/v1alpha1"
	"k8s.io/kops/util/pkg/vfs"
	"k8s.io/kubernetes/pkg/runtime/schema"
)

type EditOptions struct {
	resource.FilenameOptions
}

func NewCmdEdit(f *util.Factory, out io.Writer) *cobra.Command {

	options := &EditOptions{}
	cmd := &cobra.Command{
		Use:   "edit -f FILENAME",
		Short: "Edit a resource by filename or stdin",
		Long: `Edit a resource configuration.

This command changes the cloud specification in the registry.

It does not update the cloud resources, to apply the changes use "kops update cluster".`,
		Run: func(cmd *cobra.Command, args []string) {
			if cmdutil.IsFilenameEmpty(options.Filenames) {
				cmd.Help()
				return
			}
			cmdutil.CheckErr(RunEdit(f, cmd, out, options, args))
		},
	}

	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename to use to create the resource")
	cmd.MarkFlagRequired("filename")

	// create subcommands
	cmd.AddCommand(NewCmdEditCluster(f, out))
	cmd.AddCommand(NewCmdEditInstanceGroup(f, out))
	cmd.AddCommand(NewCmdEditFederation(f, out))

	return cmd
}

func RunEdit(f *util.Factory, cmd *cobra.Command, out io.Writer, c *EditOptions, args []string) error {

	// Codecs provides access to encoding and decoding for the scheme
	codecs := k8sapi.Codecs //serializer.NewCodecFactory(scheme)
	codec := codecs.UniversalDecoder(kopsapi.SchemeGroupVersion)

	for _, file := range c.Filenames {
		contents, err := vfs.Context.ReadFile(file)
		if err != nil {
			return fmt.Errorf("error reading file %q: %v", file, err)
		}

		sections := bytes.Split(contents, []byte("\n---\n"))

		for _, section := range sections {
			defaults := &schema.GroupVersionKind{
				Group:   v1alpha1.SchemeGroupVersion.Group,
				Version: v1alpha1.SchemeGroupVersion.Version,
			}
			o, gvk, err := codec.Decode(section, defaults, nil)
			if err != nil {
				return fmt.Errorf("error parsing file %q: %v", file, err)
			}

			switch v := o.(type) {
			case *kopsapi.Federation:
				// update federation

			case *kopsapi.Cluster:
				// updated cluster
				clusterName := v.ObjectMeta.Name
				if clusterName == "" {
					return fmt.Errorf("Config file must specify %q label with cluster name to create instanceGroup", kopsapi.LabelClusterName)
				}
				cmdutil.CheckErr(RunEditClusterFromFile(clusterName, section))

			case *kopsapi.InstanceGroup:
				clusterName := v.ObjectMeta.Labels[kopsapi.LabelClusterName]
				groupName := v.ObjectMeta.Name
				if clusterName == "" {
					return fmt.Errorf("Config file must specify %q label with cluster name to create instanceGroup", kopsapi.LabelClusterName)
				}

				if groupName == "" {
					return fmt.Errorf("Config file must specify metadata name to create instanceGroup")
				}
				cmdutil.CheckErr(RunEditInstanceGroupFromFile(clusterName, groupName, section))

			default:
				glog.V(2).Infof("Type of object was %T", v)
				return fmt.Errorf("Unhandled kind %q in %q", gvk, f)
			}
		}
	}

	return nil
}
