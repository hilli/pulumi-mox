package provider

import (
	"context"
	"fmt"
	"reflect"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/hilli/pulumi-mox/internal/moxadmin"
)

const webserverConfigID = "webserver-config"

// WebserverConfig manages mox's dynamic web redirects and web handlers.
type WebserverConfig struct{}

// WebserverConfigArgs are the user-supplied inputs.
type WebserverConfigArgs struct {
	// Redirects redirect all requests from one domain to another domain over HTTPS.
	Redirects []WebDomainRedirect `pulumi:"redirects,optional"`
	// Handlers are evaluated in order to serve, redirect, proxy, or route web requests.
	Handlers []WebHandler `pulumi:"handlers,optional"`
}

// WebDomainRedirect is a domain-to-domain HTTPS redirect.
type WebDomainRedirect struct {
	From string `pulumi:"from"`
	To   string `pulumi:"to"`
}

// WebHandler is a single mox web handler. Exactly one of Static, Redirect,
// Forward, or Internal should be set.
type WebHandler struct {
	LogName               string       `pulumi:"logName,optional"`
	Domain                string       `pulumi:"domain"`
	PathRegexp            string       `pulumi:"pathRegexp"`
	DontRedirectPlainHTTP bool         `pulumi:"dontRedirectPlainHTTP,optional"`
	Compress              bool         `pulumi:"compress,optional"`
	Static                *WebStatic   `pulumi:"static,optional"`
	Redirect              *WebRedirect `pulumi:"redirect,optional"`
	Forward               *WebForward  `pulumi:"forward,optional"`
	Internal              *WebInternal `pulumi:"internal,optional"`
}

// WebStatic serves static files.
type WebStatic struct {
	StripPrefix      string            `pulumi:"stripPrefix,optional"`
	Root             string            `pulumi:"root"`
	ListFiles        bool              `pulumi:"listFiles,optional"`
	ContinueNotFound bool              `pulumi:"continueNotFound,optional"`
	ResponseHeaders  map[string]string `pulumi:"responseHeaders,optional"`
}

// WebRedirect redirects requests to another URL or path.
type WebRedirect struct {
	BaseURL        string `pulumi:"baseUrl,optional"`
	OrigPathRegexp string `pulumi:"origPathRegexp,optional"`
	ReplacePath    string `pulumi:"replacePath,optional"`
	StatusCode     int    `pulumi:"statusCode,optional"`
}

// WebForward reverse-proxies requests.
type WebForward struct {
	StripPath       bool              `pulumi:"stripPath,optional"`
	URL             string            `pulumi:"url"`
	ResponseHeaders map[string]string `pulumi:"responseHeaders,optional"`
}

// WebInternal routes requests to an internal mox web service.
type WebInternal struct {
	BasePath string `pulumi:"basePath"`
	Service  string `pulumi:"service"`
}

// WebserverConfigState is the checkpointed output state.
type WebserverConfigState struct {
	WebserverConfigArgs
}

func (w *WebserverConfig) Annotate(a infer.Annotator) {
	a.Describe(w, "Mox dynamic webserver redirects and handlers.")
}

func (a *WebserverConfigArgs) Annotate(an infer.Annotator) {
	an.Describe(&a.Redirects, "Domain-to-domain HTTPS redirects.")
	an.Describe(&a.Handlers, "Ordered web handlers for static files, redirects, reverse proxies, or internal services.")
}

func (r *WebDomainRedirect) Annotate(an infer.Annotator) {
	an.Describe(&r.From, "Source domain to redirect from.")
	an.Describe(&r.To, "Destination domain to redirect to.")
}

func toMoxWebserverConfig(args WebserverConfigArgs) moxadmin.WebserverConfig {
	redirects := make([][2]string, 0, len(args.Redirects))
	for _, r := range args.Redirects {
		redirects = append(redirects, [2]string{r.From, r.To})
	}
	handlers := make([]moxadmin.WebHandler, 0, len(args.Handlers))
	for _, h := range args.Handlers {
		handlers = append(handlers, toMoxWebHandler(h))
	}
	return moxadmin.WebserverConfig{WebDomainRedirects: redirects, WebHandlers: handlers}
}

func fromMoxWebserverConfig(cfg moxadmin.WebserverConfig) WebserverConfigArgs {
	redirects := make([]WebDomainRedirect, 0, len(cfg.WebDNSDomainRedirects)+len(cfg.WebDomainRedirects))
	for _, r := range cfg.WebDNSDomainRedirects {
		redirects = append(redirects, WebDomainRedirect{From: r[0].ASCII, To: r[1].ASCII})
	}
	for _, r := range cfg.WebDomainRedirects {
		redirects = append(redirects, WebDomainRedirect{From: r[0], To: r[1]})
	}
	handlers := make([]WebHandler, 0, len(cfg.WebHandlers))
	for _, h := range cfg.WebHandlers {
		handlers = append(handlers, fromMoxWebHandler(h))
	}
	return WebserverConfigArgs{Redirects: redirects, Handlers: handlers}
}

func toMoxWebHandler(h WebHandler) moxadmin.WebHandler {
	return moxadmin.WebHandler{
		LogName:               h.LogName,
		Domain:                h.Domain,
		PathRegexp:            h.PathRegexp,
		DontRedirectPlainHTTP: h.DontRedirectPlainHTTP,
		Compress:              h.Compress,
		WebStatic:             toMoxWebStatic(h.Static),
		WebRedirect:           toMoxWebRedirect(h.Redirect),
		WebForward:            toMoxWebForward(h.Forward),
		WebInternal:           toMoxWebInternal(h.Internal),
	}
}

func fromMoxWebHandler(h moxadmin.WebHandler) WebHandler {
	return WebHandler{
		LogName:               h.LogName,
		Domain:                h.Domain,
		PathRegexp:            h.PathRegexp,
		DontRedirectPlainHTTP: h.DontRedirectPlainHTTP,
		Compress:              h.Compress,
		Static:                fromMoxWebStatic(h.WebStatic),
		Redirect:              fromMoxWebRedirect(h.WebRedirect),
		Forward:               fromMoxWebForward(h.WebForward),
		Internal:              fromMoxWebInternal(h.WebInternal),
	}
}

func toMoxWebStatic(s *WebStatic) *moxadmin.WebStatic {
	if s == nil {
		return nil
	}
	return &moxadmin.WebStatic{StripPrefix: s.StripPrefix, Root: s.Root, ListFiles: s.ListFiles, ContinueNotFound: s.ContinueNotFound, ResponseHeaders: s.ResponseHeaders}
}

func fromMoxWebStatic(s *moxadmin.WebStatic) *WebStatic {
	if s == nil {
		return nil
	}
	return &WebStatic{StripPrefix: s.StripPrefix, Root: s.Root, ListFiles: s.ListFiles, ContinueNotFound: s.ContinueNotFound, ResponseHeaders: s.ResponseHeaders}
}

func toMoxWebRedirect(r *WebRedirect) *moxadmin.WebRedirect {
	if r == nil {
		return nil
	}
	return &moxadmin.WebRedirect{BaseURL: r.BaseURL, OrigPathRegexp: r.OrigPathRegexp, ReplacePath: r.ReplacePath, StatusCode: r.StatusCode}
}

func fromMoxWebRedirect(r *moxadmin.WebRedirect) *WebRedirect {
	if r == nil {
		return nil
	}
	return &WebRedirect{BaseURL: r.BaseURL, OrigPathRegexp: r.OrigPathRegexp, ReplacePath: r.ReplacePath, StatusCode: r.StatusCode}
}

func toMoxWebForward(f *WebForward) *moxadmin.WebForward {
	if f == nil {
		return nil
	}
	return &moxadmin.WebForward{StripPath: f.StripPath, URL: f.URL, ResponseHeaders: f.ResponseHeaders}
}

func fromMoxWebForward(f *moxadmin.WebForward) *WebForward {
	if f == nil {
		return nil
	}
	return &WebForward{StripPath: f.StripPath, URL: f.URL, ResponseHeaders: f.ResponseHeaders}
}

func toMoxWebInternal(i *WebInternal) *moxadmin.WebInternal {
	if i == nil {
		return nil
	}
	return &moxadmin.WebInternal{BasePath: i.BasePath, Service: i.Service}
}

func fromMoxWebInternal(i *moxadmin.WebInternal) *WebInternal {
	if i == nil {
		return nil
	}
	return &WebInternal{BasePath: i.BasePath, Service: i.Service}
}

func webserverChanged(a, b WebserverConfigArgs) bool {
	return !reflect.DeepEqual(toMoxWebserverConfig(a), toMoxWebserverConfig(b))
}

func saveWebserverConfig(ctx context.Context, client *moxadmin.Client, args WebserverConfigArgs) (WebserverConfigArgs, error) {
	oldConf, err := client.WebserverConfig(ctx)
	if err != nil {
		return WebserverConfigArgs{}, fmt.Errorf("reading current webserver config: %w", err)
	}
	saved, err := client.WebserverConfigSave(ctx, oldConf, toMoxWebserverConfig(args))
	if err != nil {
		return WebserverConfigArgs{}, fmt.Errorf("saving webserver config: %w", err)
	}
	return fromMoxWebserverConfig(saved), nil
}

func (w *WebserverConfig) Create(ctx context.Context, req infer.CreateRequest[WebserverConfigArgs]) (infer.CreateResponse[WebserverConfigState], error) {
	state := WebserverConfigState{WebserverConfigArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[WebserverConfigState]{ID: webserverConfigID, Output: state}, nil
	}
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.CreateResponse[WebserverConfigState]{}, err
	}
	saved, err := saveWebserverConfig(ctx, client, req.Inputs)
	if err != nil {
		return infer.CreateResponse[WebserverConfigState]{}, err
	}
	return infer.CreateResponse[WebserverConfigState]{ID: webserverConfigID, Output: WebserverConfigState{WebserverConfigArgs: saved}}, nil
}

func (w *WebserverConfig) Read(ctx context.Context, req infer.ReadRequest[WebserverConfigArgs, WebserverConfigState]) (infer.ReadResponse[WebserverConfigArgs, WebserverConfigState], error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.ReadResponse[WebserverConfigArgs, WebserverConfigState]{}, err
	}
	cfg, err := client.WebserverConfig(ctx)
	if err != nil {
		return infer.ReadResponse[WebserverConfigArgs, WebserverConfigState]{}, fmt.Errorf("reading webserver config: %w", err)
	}
	inputs := fromMoxWebserverConfig(cfg)
	return infer.ReadResponse[WebserverConfigArgs, WebserverConfigState]{
		ID:     webserverConfigID,
		Inputs: inputs,
		State:  WebserverConfigState{WebserverConfigArgs: inputs},
	}, nil
}

func (w *WebserverConfig) Update(ctx context.Context, req infer.UpdateRequest[WebserverConfigArgs, WebserverConfigState]) (infer.UpdateResponse[WebserverConfigState], error) {
	state := WebserverConfigState{WebserverConfigArgs: req.Inputs}
	if req.DryRun {
		return infer.UpdateResponse[WebserverConfigState]{Output: state}, nil
	}
	if !webserverChanged(req.Inputs, req.State.WebserverConfigArgs) {
		return infer.UpdateResponse[WebserverConfigState]{Output: state}, nil
	}
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.UpdateResponse[WebserverConfigState]{}, err
	}
	saved, err := saveWebserverConfig(ctx, client, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[WebserverConfigState]{}, err
	}
	return infer.UpdateResponse[WebserverConfigState]{Output: WebserverConfigState{WebserverConfigArgs: saved}}, nil
}

func (w *WebserverConfig) Delete(ctx context.Context, req infer.DeleteRequest[WebserverConfigState]) (infer.DeleteResponse, error) {
	client, err := clientFromContext(ctx)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if _, err := saveWebserverConfig(ctx, client, WebserverConfigArgs{}); err != nil {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
