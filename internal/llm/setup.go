package llm

import (
	"fmt"

	"github.com/samautomateai/tessivo/internal/config"
	"github.com/samautomateai/tessivo/internal/secrets"
)

// NewRouter builds a Router for a resolved alias, creating one client per
// provider and looking up API keys through sec.
func NewRouter(f config.File, alias config.Alias, sec secrets.Resolver) (*Router, error) {
	clients := map[string]ProviderClient{}
	r := &Router{Triggers: alias.Triggers(), MaxRetries: alias.MaxRetries}
	for _, m := range alias.Chain {
		c, ok := clients[m.Provider]
		if !ok {
			p, found := f.Provider(m.Provider)
			if !found {
				return nil, fmt.Errorf("alias %q: unknown provider %q", alias.Name, m.Provider)
			}
			key := ""
			if p.APIKeyRef != "" {
				v, err := sec.Get(p.APIKeyRef)
				if err != nil {
					return nil, fmt.Errorf("provider %q api key: %w", p.Name, err)
				}
				key = v
			}
			c = NewProviderClient(p, key)
			clients[m.Provider] = c
		}
		r.Chain = append(r.Chain, Target{Label: m.Provider + "/" + m.Model, Model: m.Model, Client: c})
	}
	return r, nil
}
