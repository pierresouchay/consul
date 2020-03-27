package consul

import (
	"fmt"
	"sort"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	bexpr "github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-memdb"
)

// Health endpoint is used to query the health information
type Health struct {
	srv *Server
}

// ChecksInState is used to get all the checks in a given state
func (h *Health) ChecksInState(args *structs.ChecksInStateRequest,
	reply *structs.IndexedHealthChecks) error {
	if done, err := h.srv.forward("Health.ChecksInState", args, args, reply); done {
		return err
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.HealthChecks)
	if err != nil {
		return err
	}

	_, err = h.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := h.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return h.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var checks structs.HealthChecks
			var err error
			if len(args.NodeMetaFilters) > 0 {
				index, checks, err = state.ChecksInStateByNodeMeta(ws, args.State, args.NodeMetaFilters, &args.EnterpriseMeta)
			} else {
				index, checks, err = state.ChecksInState(ws, args.State, &args.EnterpriseMeta)
			}
			if err != nil {
				return err
			}
			reply.Index, reply.HealthChecks = index, checks
			if err := h.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			raw, err := filter.Execute(reply.HealthChecks)
			if err != nil {
				return err
			}
			reply.HealthChecks = raw.(structs.HealthChecks)

			return h.srv.sortNodesByDistanceFrom(args.Source, reply.HealthChecks)
		})
}

// NodeChecks is used to get all the checks for a node
func (h *Health) NodeChecks(args *structs.NodeSpecificRequest,
	reply *structs.IndexedHealthChecks) error {
	if done, err := h.srv.forward("Health.NodeChecks", args, args, reply); done {
		return err
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.HealthChecks)
	if err != nil {
		return err
	}

	_, err = h.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := h.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return h.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, checks, err := state.NodeChecks(ws, args.Node, &args.EnterpriseMeta)
			if err != nil {
				return err
			}
			reply.Index, reply.HealthChecks = index, checks
			if err := h.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			raw, err := filter.Execute(reply.HealthChecks)
			if err != nil {
				return err
			}
			reply.HealthChecks = raw.(structs.HealthChecks)
			return nil
		})
}

// ServiceChecks is used to get all the checks for a service
func (h *Health) ServiceChecks(args *structs.ServiceSpecificRequest,
	reply *structs.IndexedHealthChecks) error {

	// Reject if tag filtering is on
	if args.TagFilter {
		return fmt.Errorf("Tag filtering is not supported")
	}

	// Potentially forward
	if done, err := h.srv.forward("Health.ServiceChecks", args, args, reply); done {
		return err
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.HealthChecks)
	if err != nil {
		return err
	}

	_, err = h.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if err := h.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return h.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var checks structs.HealthChecks
			var err error
			if len(args.NodeMetaFilters) > 0 {
				index, checks, err = state.ServiceChecksByNodeMeta(ws, args.ServiceName, args.NodeMetaFilters, &args.EnterpriseMeta)
			} else {
				index, checks, err = state.ServiceChecks(ws, args.ServiceName, &args.EnterpriseMeta)
			}
			if err != nil {
				return err
			}
			reply.Index, reply.HealthChecks = index, checks
			if err := h.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			raw, err := filter.Execute(reply.HealthChecks)
			if err != nil {
				return err
			}
			reply.HealthChecks = raw.(structs.HealthChecks)

			return h.srv.sortNodesByDistanceFrom(args.Source, reply.HealthChecks)
		})
}

// ServiceNodes returns all the nodes registered as part of a service including health info.
//
// swagger:operation GET /health/service/{service} health-for-service
//
// This endpoint returns the nodes providing the service indicated on the path.
// Users can also build in support for dynamic load balancing and other features by incorporating the use of health checks.
//
// ---
// produces:
// - application/json
// parameters:
// - name: service
//   in: path
//   type: string
//   description: Service name to get instances for
//   required: true
// - name: dc
//   in: query
//   type: string
//   description: Specifies the datacenter to query. This will default to the datacenter of the agent being queried.
// - name: near
//   in: query
//   type: string
//   description: Specifies a node name to sort the node list in ascending order based on the estimated round trip time from that node. Passing ?near=_agent will use the agent's node for the sort.
// - name: stale
//   in: query
//   type: boolean
//   description: Can the request
// responses:
//   '200':
//     description: 'OK'
//     type: array
//     schema:
//       $ref: '#/definitions/CheckServiceNode'
//     headers:
//       X-Consul-Index:
//         type: integer
//         format: uint64
//         description: Last Consul Index for this endpoint
//       X-Consul-KnownLeader:
//         type: boolean
//         description: true if a Consul server has been elected as leader
//       X-Consul-Effective-Consistency:
//         type: string
//         enum: [consistent,leader,stale]
//         description: Consistencylevel returns the consistency used to serve the query
//       X-Consul-LastContact:
//         type: integer
//         format: uint64
//         description: Latency of servers with current leader in ms
func (h *Health) ServiceNodes(args *structs.ServiceSpecificRequest, reply *structs.IndexedCheckServiceNodes) error {
	if done, err := h.srv.forward("Health.ServiceNodes", args, args, reply); done {
		return err
	}

	// Verify the arguments
	if args.ServiceName == "" {
		return fmt.Errorf("Must provide service name")
	}

	// Determine the function we'll call
	var f func(memdb.WatchSet, *state.Store, *structs.ServiceSpecificRequest) (uint64, structs.CheckServiceNodes, error)
	switch {
	case args.Connect:
		f = h.serviceNodesConnect
	case args.TagFilter:
		f = h.serviceNodesTagFilter
	default:
		f = h.serviceNodesDefault
	}

	var authzContext acl.AuthorizerContext
	authz, err := h.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := h.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	// If we're doing a connect query, we need read access to the service
	// we're trying to find proxies for, so check that.
	if args.Connect {
		if authz != nil && authz.ServiceRead(args.ServiceName, &authzContext) != acl.Allow {
			// Just return nil, which will return an empty response (tested)
			return nil
		}
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.Nodes)
	if err != nil {
		return err
	}

	err = h.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, nodes, err := f(ws, state, args)
			if err != nil {
				return err
			}

			reply.Index, reply.Nodes = index, nodes
			if len(args.NodeMetaFilters) > 0 {
				reply.Nodes = nodeMetaFilter(args.NodeMetaFilters, reply.Nodes)
			}

			if err := h.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			raw, err := filter.Execute(reply.Nodes)
			if err != nil {
				return err
			}
			reply.Nodes = raw.(structs.CheckServiceNodes)

			return h.srv.sortNodesByDistanceFrom(args.Source, reply.Nodes)
		})

	// Provide some metrics
	if err == nil {
		// For metrics, we separate Connect-based lookups from non-Connect
		key := "service"
		if args.Connect {
			key = "connect"
		}

		metrics.IncrCounterWithLabels([]string{"health", key, "query"}, 1,
			[]metrics.Label{{Name: "service", Value: args.ServiceName}})
		// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
		// with 1.2.x is not required.
		if args.ServiceTag != "" {
			metrics.IncrCounterWithLabels([]string{"health", key, "query-tag"}, 1,
				[]metrics.Label{{Name: "service", Value: args.ServiceName}, {Name: "tag", Value: args.ServiceTag}})
		}
		if len(args.ServiceTags) > 0 {
			// Sort tags so that the metric is the same even if the request
			// tags are in a different order
			sort.Strings(args.ServiceTags)

			labels := []metrics.Label{{Name: "service", Value: args.ServiceName}}
			for _, tag := range args.ServiceTags {
				labels = append(labels, metrics.Label{Name: "tag", Value: tag})
			}
			metrics.IncrCounterWithLabels([]string{"health", key, "query-tags"}, 1, labels)
		}
		if len(reply.Nodes) == 0 {
			metrics.IncrCounterWithLabels([]string{"health", key, "not-found"}, 1,
				[]metrics.Label{{Name: "service", Value: args.ServiceName}})
		}
	}
	return err
}

// The serviceNodes* functions below are the various lookup methods that
// can be used by the ServiceNodes endpoint.

func (h *Health) serviceNodesConnect(ws memdb.WatchSet, s *state.Store, args *structs.ServiceSpecificRequest) (uint64, structs.CheckServiceNodes, error) {
	return s.CheckConnectServiceNodes(ws, args.ServiceName, &args.EnterpriseMeta)
}

func (h *Health) serviceNodesTagFilter(ws memdb.WatchSet, s *state.Store, args *structs.ServiceSpecificRequest) (uint64, structs.CheckServiceNodes, error) {
	// DEPRECATED (singular-service-tag) - remove this when backwards RPC compat
	// with 1.2.x is not required.
	// Agents < v1.3.0 populate the ServiceTag field. In this case,
	// use ServiceTag instead of the ServiceTags field.
	if args.ServiceTag != "" {
		return s.CheckServiceTagNodes(ws, args.ServiceName, []string{args.ServiceTag}, &args.EnterpriseMeta)
	}
	return s.CheckServiceTagNodes(ws, args.ServiceName, args.ServiceTags, &args.EnterpriseMeta)
}

func (h *Health) serviceNodesDefault(ws memdb.WatchSet, s *state.Store, args *structs.ServiceSpecificRequest) (uint64, structs.CheckServiceNodes, error) {
	return s.CheckServiceNodes(ws, args.ServiceName, &args.EnterpriseMeta)
}
