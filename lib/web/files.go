/*
Copyright 2015 Gravitational, Inc.

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

package web

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gravitational/form"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/teleport/lib/sshutils/scp"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/crypto/ssh"
)

type TransferFileRequest struct {
	// Server describes a server to connect to (serverId|hostname[:port]).
	Server string `json:"server_id"`
	// Login is Linux username to connect as.
	Login string `json:"login"`
	// Namespace is node namespace.
	Namespace string `json:"namespace"`
	// Cluster is the name of the remote cluster to connect to.
	Cluster string `json:"-"`
	// Cluster is the name of the remote cluster to connect to.
	UploadLocation string `json:"uploadLocation"`
}

// changePassword updates users password based on the old password
func (h *Handler) uploadFile(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	uploadLocationPath := r.URL.Query().Get("path")
	req := TransferFileRequest{
		Login:     p.ByName("login"),
		Namespace: p.ByName("namespace"),
		Server:    p.ByName("node"),
	}

	files, err := parseFiles(r, "file")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tc, err := h.createSCPClient(req, clt, ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uploadLocationPath = filepath.Join(uploadLocationPath, files[0].Name())

	cmd, err := scp.CreateHTTPUploadCommand(uploadLocationPath, files[0], tc.Stdout)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = tc.RunSCPCommand(context.TODO(), cmd)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil
}

//type TransferFileRequest struct {
//}

func (h *Handler) downloadFile(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *SessionContext, site reversetunnel.RemoteSite) (interface{}, error) {
	filePath := r.URL.Query().Get("path")
	req := TransferFileRequest{
		Login:     p.ByName("login"),
		Namespace: p.ByName("namespace"),
		Server:    p.ByName("node"),
	}

	clt, err := ctx.GetUserClient(site)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tc, err := h.createSCPClient(req, clt, ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cmd, err := scp.CreateHTTPDownloadCommand(filePath, w, tc.Stdout)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = tc.RunSCPCommand(context.TODO(), cmd)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return ok(), nil

}

func (h *Handler) createSCPClient(req TransferFileRequest, authProvider AuthProvider, ctx *SessionContext) (*client.TeleportClient, error) {
	if !services.IsValidNamespace(req.Namespace) {
		return nil, trace.BadParameter("invalid namespace %q", req.Namespace)
	}

	if req.Login == "" {
		return nil, trace.BadParameter("login: missing login")
	}

	servers, err := authProvider.GetNodes(req.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostName, hostPort, err := resolveServerHostPort(req.Server, servers)
	if err != nil {
		return nil, trace.BadParameter("invalid server name %q: %v", req.Server, err)
	}

	agent, cert, err := ctx.GetAgent()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	signers, err := agent.Signers()
	if err != nil {
		return nil, trace.BadParameter("failed to get user credentials: %v", err)
	}

	tlsConfig, err := ctx.ClientTLSConfig()
	if err != nil {
		return nil, trace.BadParameter("failed to get client TLS config: %v", err)
	}

	clientConfig := &client.Config{
		HostLogin:        req.Login,
		SiteName:         req.Cluster,
		Namespace:        req.Namespace,
		SkipLocalAuth:    true,
		TLS:              tlsConfig,
		AuthMethods:      []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		DefaultPrincipal: cert.ValidPrincipals[0],
		Username:         ctx.user,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
		Stdin:            os.Stdin,
		ProxyHostPort:    h.ProxyHostPort(),
		Host:             hostName,
		HostPort:         hostPort,
		HostKeyCallback:  func(string, net.Addr, ssh.PublicKey) error { return nil },
	}

	tc, err := client.NewClient(clientConfig)
	if err != nil {
		return nil, trace.BadParameter("failed to create client: %v", err)
	}

	return tc, nil
}

// readFile reads the file by the provided name from the request and
// returns its content
func parseFiles(r *http.Request, name string) (form.Files, error) {
	var files form.Files
	err := form.Parse(r, form.FileSlice(name, &files))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(files) == 0 {
		return nil, trace.NotFound("file %q is not provided", name)
	}
	if len(files) != 1 {
		return nil, trace.BadParameter("expected 1 file %q, got %v", name, len(files))
	}

	return files, nil

}
