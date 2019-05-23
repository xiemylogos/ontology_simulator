// p2psim provides a command-line client for a simulation HTTP API.
//
// Here is an example of creating a 2 node network with the first node
// connected to the second:
//
//     $ p2psim node create
//     Created node01
//
//     $ p2psim node start node01
//     Started node01
//
//     $ p2psim node create
//     Created node02
//
//     $ p2psim node start node02
//     Started node02
//
//     $ p2psim node connect node01 node02
//     Connected node01 to node02
//
package p2psim

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ontio/ontology/account"
	"github.com/ontio/ontology/common/log"
	"github.com/ontio/ontology/core/signature"
	http "github.com/ontio/ontology_simulator/http"
	types "github.com/ontio/ontology_simulator/p2pserver/peer"
	"gopkg.in/urfave/cli.v1"
)

var client *http.Client

func main() {
	log.Init(log.Stdout)
	log.Log.SetDebugLevel(1)

	rand.Seed(time.Now().UnixNano())
	//crypto.SetAlg("P256R1")

	app := cli.NewApp()
	app.Usage = "devp2p simulation command-line client"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "api",
			Value:  "http://localhost:8888",
			Usage:  "simulation API URL",
			EnvVar: "P2PSIM_API_URL",
		},
	}
	app.Before = func(ctx *cli.Context) error {
		client = http.NewClient(ctx.GlobalString("api"))
		return nil
	}
	app.Commands = []cli.Command{
		{
			Name:   "show",
			Usage:  "show network information",
			Action: showNetwork,
		},
		{
			Name:   "node",
			Usage:  "manage simulation nodes",
			Action: listNodes,
			Subcommands: []cli.Command{
				{
					Name:   "list",
					Usage:  "list nodes",
					Action: listNodes,
				},
				{
					Name:   "create",
					Usage:  "create a node",
					Action: createNode,
					Flags: []cli.Flag{
						cli.StringFlag{
							Name:  "name",
							Value: "",
							Usage: "node name",
						},
						cli.StringFlag{
							Name:  "services",
							Value: "",
							Usage: "node services (comma separated)",
						},
						cli.StringFlag{
							Name:  "wallet",
							Value: "",
							Usage: "wallet path",
						},
					},
				},
				{
					Name:      "start",
					ArgsUsage: "<node>",
					Usage:     "start a node",
					Action:    startNode,
				},
				{
					Name:      "stop",
					ArgsUsage: "<node>",
					Usage:     "stop a node",
					Action:    stopNode,
				},
				{
					Name:      "connect",
					ArgsUsage: "<node> <peer>",
					Usage:     "connect a node to a peer node",
					Action:    connectNode,
				},
				{
					Name:      "disconnect",
					ArgsUsage: "<node> <peer>",
					Usage:     "disconnect a node from a peer node",
					Action:    disconnectNode,
				},
			},
		},
	}
	app.Run(os.Args)
}

func showNetwork(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	network, err := client.GetNetwork()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(ctx.App.Writer, 1, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NODES\t%d\n", len(network.Nodes))
	fmt.Fprintf(w, "CONNS\t%d\n", len(network.Conns))
	return nil
}

func listNodes(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodes, err := client.GetNodes()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(ctx.App.Writer, 1, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NAME\tPROTOCOLS\tID\n")
	for _, node := range nodes {
		fmt.Fprintf(w, "%s\t%s\t%s\n", node)
	}
	return nil
}

func createNode(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	config := &types.NodeConfig{}

	if walletpath := ctx.String("wallet"); walletpath != "" {
		wallet, err := account.Open(walletpath)
		if err != nil {
			return err
		}
		acc, err := wallet.GetDefaultAccount([]byte("123456"))
		if err != nil {
			return err
		}
		config.ID = types.PubkeyID(acc.PublicKey)
		config.PrivateKey = acc.PrivateKey

		if err := testKeyPair(acc, config.ID); err != nil {
			return fmt.Errorf("test keypair failed: %s", err)
		}
	}
	if services := ctx.String("services"); services != "" {
		config.Services = strings.Split(services, ",")
	}
	node, err := client.CreateNode(config)
	if err != nil {
		fmt.Fprintln(ctx.App.Writer, "Created failed", err)
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Created", node)
	return nil
}

func startNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 1 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	if err := client.StartNode(nodeName); err != nil {
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Started", nodeName)
	return nil
}

func stopNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 1 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	if err := client.StopNode(nodeName); err != nil {
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Stopped", nodeName)
	return nil
}

func connectNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 2 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	peerName := args[1]
	if err := client.ConnectNode(nodeName, peerName); err != nil {
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Connected", nodeName, "to", peerName)
	return nil
}

func disconnectNode(ctx *cli.Context) error {
	args := ctx.Args()
	if len(args) != 2 {
		return cli.ShowCommandHelp(ctx, ctx.Command.Name)
	}
	nodeName := args[0]
	peerName := args[1]
	if err := client.DisconnectNode(nodeName, peerName); err != nil {
		return err
	}
	fmt.Fprintln(ctx.App.Writer, "Disconnected", nodeName, "from", peerName)
	return nil
}

func testKeyPair(sk *account.Account, id string) error {
	testData := []byte("test data")
	sig, err := signature.Sign(sk, testData)
	if err != nil {
		return fmt.Errorf("sign failed: %s", err)
	}
	pk, err := types.Pubkey(id)
	if err != nil {
		return fmt.Errorf("get pk failed: %s", err)
	}
	if err := signature.Verify(pk, testData, sig); err != nil {
		return fmt.Errorf("verify failed: %s", err)
	}
	return nil
}
