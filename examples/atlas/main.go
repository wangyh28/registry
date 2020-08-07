// Copyright 2020 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/apigee/registry/connection"
	rpcpb "github.com/apigee/registry/rpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const directory = "."
const project = "atlas"

var registryClient connection.Client

func notFound(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.NotFound
}

func alreadyExists(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.AlreadyExists
}

func main() {
	var err error

	ctx := context.Background()
	registryClient, err = connection.NewClient(ctx)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(-1)
	}
	completions := make(chan int)
	processes := 0

	// walk a directory hierarchy, uploading every API spec that matches a set of expected file names.
	err = filepath.Walk(directory,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if strings.HasSuffix(path, "swagger.yaml") || strings.HasSuffix(path, "swagger.json") {
				processes++
				go func() {
					err := handleSpec(path, "openapi/v2")
					if err != nil {
						fmt.Printf("%s\n", err.Error())
					}
					completions <- 1
				}()
			}
			if strings.HasSuffix(path, "openapi.yaml") || strings.HasSuffix(path, "openapi.json") {
				processes++
				go func() {
					err := handleSpec(path, "openapi/v3")
					if err != nil {
						fmt.Printf("%s\n", err.Error())
					}
					completions <- 1
				}()
			}
			return nil
		})
	if err != nil {
		log.Println(err)
	}
	for i := 0; i < processes; i++ {
		<-completions
		fmt.Printf("COMPLETE: %d\n", i+1)
	}
}

func handleSpec(path string, style string) error {
	// Compute the API name from the path to the spec file.
	name := strings.TrimPrefix(path, directory)
	parts := strings.Split(name, "/")
	spec := parts[len(parts)-1]
	version := parts[len(parts)-2]
	api := strings.Join(parts[0:len(parts)-2], "/")
	fmt.Printf("api:%+v version:%+v spec:%+v \n", api, version, spec)
	// Upload the spec for the specified api, version, and style
	return uploadSpec(api, version, style, path)
}

func uploadSpec(apiName, version, style, path string) error {
	ctx := context.TODO()
	api := strings.Replace(apiName, "/", "-", -1)
	// If the API does not exist, create it.
	{
		request := &rpcpb.GetApiRequest{}
		request.Name = "projects/" + project + "/apis/" + api
		_, err := registryClient.GetApi(ctx, request)
		if notFound(err) {
			request := &rpcpb.CreateApiRequest{}
			request.Parent = "projects/" + project
			request.ApiId = api
			request.Api = &rpcpb.Api{}
			request.Api.DisplayName = apiName
			response, err := registryClient.CreateApi(ctx, request)
			if err == nil {
				log.Printf("created %s", response.Name)
			} else if alreadyExists(err) {
				log.Printf("already exists %s/apis/%s", request.Parent, request.ApiId)
			} else {
				log.Printf("failed to create %s/apis/%s: %s",
					request.Parent, request.ApiId, err.Error())
			}
		} else if err != nil {
			return err
		}
	}
	// If the API version does not exist, create it.
	{
		request := &rpcpb.GetVersionRequest{}
		request.Name = "projects/" + project + "/apis/" + api + "/versions/" + version
		_, err := registryClient.GetVersion(ctx, request)
		if notFound(err) {
			request := &rpcpb.CreateVersionRequest{}
			request.Parent = "projects/" + project + "/apis/" + api
			request.VersionId = version
			request.Version = &rpcpb.Version{}
			response, err := registryClient.CreateVersion(ctx, request)
			if err == nil {
				log.Printf("created %s", response.Name)
			} else if alreadyExists(err) {
				log.Printf("already exists %s/versions/%s", request.Parent, request.VersionId)
			} else {
				log.Printf("failed to create %s/versions/%s: %s",
					request.Parent, request.VersionId, err.Error())
			}
		} else if err != nil {
			return err
		}
	}
	// If the API spec does not exist, create it.
	{
		filename := filepath.Base(path)

		request := &rpcpb.GetSpecRequest{}
		request.Name = "projects/" + project + "/apis/" + api +
			"/versions/" + version +
			"/specs/" + filename
		_, err := registryClient.GetSpec(ctx, request)
		if notFound(err) {
			fileBytes, err := ioutil.ReadFile(path)

			// gzip the spec before uploading it
			var buf bytes.Buffer
			zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
			_, err = zw.Write(fileBytes)
			if err != nil {
				log.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				log.Fatal(err)
			}

			request := &rpcpb.CreateSpecRequest{}
			request.Parent = "projects/" + project + "/apis/" + api +
				"/versions/" + version
			request.SpecId = filename
			request.Spec = &rpcpb.Spec{}
			request.Spec.Style = style + "+gzip"
			request.Spec.Filename = filename
			request.Spec.Contents = buf.Bytes()
			response, err := registryClient.CreateSpec(ctx, request)
			if err == nil {
				log.Printf("created %s", response.Name)
			} else if alreadyExists(err) {
				log.Printf("already exists %s/specs/%s", request.Parent, request.SpecId)
			} else {
				details := fmt.Sprintf("contents-length: %d", len(request.Spec.Contents))
				log.Printf("failed to create %s/specs/%s: %s [%s]",
					request.Parent, request.SpecId, err.Error(), details)
			}
		} else if err != nil {
			return err
		}
	}
	return nil
}