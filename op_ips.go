package spruce

import (
	"fmt"
	"net"

	"github.com/ziutek/utils/netaddr"

	"github.com/starkandwayne/goutils/tree"

	. "github.com/alicegray33/bruce/log"
)

// IpOperator...
type IpsOperator struct{}

// Setup ...
func (IpsOperator) Setup() error {
	return nil
}

// Phase ...
func (IpsOperator) Phase() OperatorPhase {
	return EvalPhase
}

// Dependencies ...
func (IpsOperator) Dependencies(_ *Evaluator, args []*Expr, locs []*tree.Cursor, auto []*tree.Cursor) []*tree.Cursor {
	l := []*tree.Cursor{}

	for _, arg := range args {
		if arg.Type != Reference {
			continue
		}

		for _, other := range locs {
			if other.Under(arg.Reference) {
				l = append(l, other)
			}
		}
	}

	//append autogenerated dependencies (operator reference-type arguments)
	for _, dep := range auto {
		l = append(l, dep)
	}

	return l
}

func makeInt(val interface{}) int {
	var num int

	num, ok := val.(int)
	if !ok {
		num = int(val.(int64))
	}
	return num
}

func netSize(ipnet *net.IPNet) int {
	ones, bits := ipnet.Mask.Size()
	return 1 << uint(bits-ones)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// Run ...
func (IpsOperator) Run(ev *Evaluator, args []*Expr) (*Response, error) {
	DEBUG("running (( ips ... )) operation at $.%s", ev.Here)
	defer DEBUG("done with (( ips ... )) operation at $%s\n", ev.Here)

	if len(args) < 2 {
		return nil, fmt.Errorf("ips requires at least two arguments: 1) An IP or a CIDR and 2) an index")
	}

	var vals []interface{}

	for i, arg := range args {
		v, err := arg.Resolve(ev.Tree)

		if err != nil {
			DEBUG("     [%d]: resolution failed\n    error: %s", i, err)
			return nil, err
		}

		switch v.Type {
		case Literal:
			DEBUG("  arg[%d]: found string literal '%s'", i, v.Literal)
			vals = append(vals, v.Literal)

		case Reference:
			DEBUG("  arg[%d]: trying to resolve reference $.%s", i, v.Reference)
			s, err := v.Reference.Resolve(ev.Tree)
			if err != nil {
				DEBUG("     [%d]: resolution failed\n    error: %s", i, err)
				return nil, fmt.Errorf("Unable to resolve `%s`: %s", v.Reference, err)
			}
			DEBUG("     [%d]: resolved to a value (could be a map, a list or a scalar); appending", i)
			vals = append(vals, s)

		default:
			DEBUG("  arg[%d]: I don't know what to do with '%v'", i, arg)
			return nil, fmt.Errorf("ips operator only accepts literals and key reference arguments")
		}
		DEBUG("")
	}

	ip, ipnet, err := net.ParseCIDR(vals[0].(string))
	if err != nil {
		ip = net.ParseIP(vals[0].(string))
		if ip == nil {
			DEBUG("     [n]: failed to parse IP or CIDR \"%s\": %s", vals[0], err)
			return nil, err
		}
	}

	start := makeInt(vals[1])

	if ipnet != nil {
		ip = ip.Mask(ipnet.Mask)
		netsize := netSize(ipnet)

		if abs(start) > netsize {
			return nil, fmt.Errorf("Start index %d exceeds size of subnet %s", start, vals[0])
		}
		if start < 0 {
			start += netsize
		}
	}

	if len(args) == 2 {
		return &Response{
			Type:  Replace,
			Value: netaddr.IPAdd(ip, start).String(),
		}, nil
	} else {
		count := makeInt(vals[2])
		if ipnet != nil {
			if start+count > netSize(ipnet) {
				return nil, fmt.Errorf("Start index %d and count %d would exceed size of subnet %s", start, count, vals[0])
			}
		}
		lst := []interface{}{}
		for i := start; i < start+count; i++ {
			lst = append(lst, netaddr.IPAdd(ip, i).String())
		}
		return &Response{
			Type:  Replace,
			Value: lst,
		}, nil
	}
}

func init() {
	RegisterOp("ips", IpsOperator{})
}
