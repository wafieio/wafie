package relay

import (
	"context"
	"fmt"

	wv1 "github.com/wafieio/wafie/api/gen/wafie/v1"
	"sigs.k8s.io/knftables"
)

type (
	operation string
)

const (
	WafieGatewayNatTable                  = "appsecgw"
	WafieGatewayPreroutingChain           = "prerouting"
	WafieOwnedComment                     = "appsecgw-owned-object"
	AppSecGwIpsSet                        = "appsecgw-ips-set"
	AddOp                       operation = "add"
	DeleteOp                    operation = "delete"
)

func ProgramNft(op operation, options *wv1.RelayOptions) error {
	nft, err := knftables.New(knftables.InetFamily, WafieGatewayNatTable)
	if err != nil {
		return err
	}

	tx := nft.NewTransaction()
	// create nft rules
	if op == AddOp {
		rulesApplied, err := rulesState(nft)
		if err != nil {
			return err
		}
		if !rulesApplied {
			add(tx, options)
		}
	}
	// delete nft rules
	if op == DeleteOp {
		rulesApplied, err := rulesState(nft)
		if err != nil {
			return err
		}
		if rulesApplied {
			remove(tx)
		}
	}

	return nft.Run(context.Background(), tx)
}
func state(nft knftables.Interface) (obtained bool, err error) {
	ruleState, err := rulesState(nft)
	if err != nil {
		return true, err // on error do nothing
	}
	setState, err := setsState(nft)
	if err != nil {
		return true, err // on error do nothing
	}
	// is desired state obtained
	return ruleState && setState, nil
}

func setsState(nft knftables.Interface) (applied bool, err error) {
	set, err := nft.ListElements(context.Background(), "set", AppSecGwIpsSet)
	if err != nil {
		return true, err
	}
	for _, e := range set {
		fmt.Println(e.Key)
	}
	return false, err
}

func rulesState(nft knftables.Interface) (applied bool, err error) {
	chains, err := nft.List(context.Background(), "chains")
	// in case of error, do not program anything
	if err != nil {
		return true, err
	}
	// if no chains exists, program is required
	if len(chains) == 0 {
		return false, nil
	}
	// if chains list includes WafieGatewayPreroutingChain, further checks are required
	for _, chain := range chains {
		// first make sure the chain with the WafieGatewayPreroutingChain name exists
		if chain == WafieGatewayPreroutingChain {
			// list all rules in the WafieGatewayPreroutingChain chain
			rules, err := nft.ListRules(context.Background(), WafieGatewayPreroutingChain)
			// in case of error, do not program anything
			if err != nil {
				return true, err
			}
			// if no rules are found in the chain, program is required
			if len(rules) == 0 {
				return false, nil
			}
			// make sure the chain have at least one rule with WafieOwnedComment comment
			for _, rule := range rules {
				if *rule.Comment == WafieOwnedComment {
					return true, nil
				}
			}
			return false, nil
		}
	}
	// program required
	return false, nil

}

func add(tx *knftables.Transaction, options *wv1.RelayOptions) {
	// add table
	tx.Add(table())
	// add chain
	tx.Add(chain())
	// add set
	tx.Add(set())
	// add proxy ip to set
	tx.Add(ipElement(options.ProxyIp))
	// add rules
	tx.Add(rule(options))

}

func remove(tx *knftables.Transaction) {
	tx.Delete(table())
}

func ipElement(ip string) *knftables.Element {
	return &knftables.Element{
		Set: AppSecGwIpsSet,
		Key: []string{ip},
	}
}

func set() *knftables.Set {
	comment := WafieOwnedComment
	return &knftables.Set{
		Name:    AppSecGwIpsSet,
		Type:    "ipv4_addr",
		Comment: &comment,
	}
}

func table() *knftables.Table {
	return &knftables.Table{
		Family: knftables.InetFamily,
		Name:   WafieGatewayNatTable,
	}
}

func chain() *knftables.Chain {
	comment := WafieOwnedComment
	return &knftables.Chain{
		Name:     WafieGatewayPreroutingChain,
		Table:    WafieGatewayNatTable,
		Family:   knftables.InetFamily,
		Type:     knftables.PtrTo(knftables.NATType),
		Hook:     knftables.PtrTo(knftables.PreroutingHook),
		Priority: knftables.PtrTo(knftables.DNATPriority),
		Comment:  &comment,
	}
}

// iptables -t nat -A PREROUTING -p tcp --dport 80 ! -s 192.168.1.100 -j DNAT --to-destination 10.0.0.10:8080
// nft replace rule inet nat prerouting ip saddr != 10.244.0.29 tcp dport 8080 redirect to :9090 comment "wafie-owned-object"

func rule(options *wv1.RelayOptions) *knftables.Rule {
	comment := WafieOwnedComment
	return &knftables.Rule{
		Table:   WafieGatewayNatTable,
		Chain:   WafieGatewayPreroutingChain,
		Comment: &comment,
		Rule: knftables.Concat(
			"ip saddr != @", AppSecGwIpsSet,
			"tcp dport", options.AppContainerPort,
			"redirect to :", options.RelayPort,
		),
	}
}

//
//
//# Create a set for allowed IPs
//nft add set inet filter allowed_ips { type ipv4_addr\; }
//
//# Add IPs to the set
//nft add element inet filter allowed_ips { 192.168.1.10, 192.168.1.20, 10.0.0.5 }
//
//# Create rule using the set
//nft add rule inet filter input ip saddr @allowed_ips accept
//
//Create a set for blocked IPs:
//
//# Create set for blocked IPs
//nft add set inet filter blocked_ips { type ipv4_addr\; }
//
//# Add IPs to the set
//nft add element inet filter blocked_ips { 192.168.1.100, 192.168.1.101, 10.0.0.99 }
//
//# Create rule to drop traffic from blocked IPs
//nft add rule inet filter input ip saddr @blocked_ips drop
