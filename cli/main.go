package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ghetzel/cli"
	"github.com/ghetzel/onkyo-remote"
	"github.com/op/go-logging"
)

var log = logging.MustGetLogger(`main`)

// func (s shell) printInfo() {
// 	log.Println(s.r.Info())
// 	must(s.r.Send("PWR", "QSTN"))
// 	must(s.r.Send("AMT", "QSTN"))
// 	must(s.r.Send("MVL", "QSTN"))
// 	must(s.r.Send("SLP", "QSTN"))
// 	must(s.r.Send("DIM", "QSTN"))
// 	must(s.r.Send("IFA", "QSTN"))
// 	must(s.r.Send("SLI", "QSTN"))
// 	<-time.After(1 * time.Second)
// }

var device *onkyo.Device

func configureDevices(c *cli.Context) error {
	if devices, err := onkyo.Discover(c.Duration(`discovery-timeout`), c.String(`host`)); err == nil {
		for _, d := range devices {
			info := d.Info()
			log.Noticef("Found device: [%x] %s at %s", info.Identifier, info.Model, d.Address().String())
		}

		if len(devices) > 0 {
			device = devices[0]
			return nil
		} else {
			return fmt.Errorf("No devices found.")
		}
	} else {
		return fmt.Errorf("Failed to auto-discover devices: %v", err)
	}
}

func main() {
	app := cli.NewApp()
	app.Name = `onkyo-remote`
	app.Version = `0.0.1`
	app.EnableBashCompletion = false

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  `log-level, L`,
			Usage: `The level of logging verbosity to output.`,
			Value: `error`,
		},
		cli.StringFlag{
			Name:   `host, H`,
			Usage:  `The IP address of the Onkyo ISCP device to control (use "auto" to auto-discover)`,
			EnvVar: `ONKYO_ISCP_HOST`,
			Value:  `auto`,
		},
		cli.DurationFlag{
			Name:  `discovery-timeout, T`,
			Usage: `How long to perform auto-discovery for`,
			Value: onkyo.DEFAULT_DISCOVERY_TIMEOUT,
		},
		cli.DurationFlag{
			Name:  `response-timeout, R`,
			Usage: `How long to wait for command responses`,
			Value: onkyo.DEFAULT_RESPONSE_TIMEOUT,
		},
	}

	app.Before = func(c *cli.Context) error {
		logging.SetFormatter(logging.MustStringFormatter(`%{color}%{level:.4s}%{color:reset}[%{id:04d}] %{message}`))

		if level, err := logging.LogLevel(c.String(`log-level`)); err == nil {
			logging.SetLevel(level, `main`)
			logging.SetLevel(level, `onkyo`)
		}

		initCommands()

		switch c.Args().First() {
		case `help`: // don't go through discovery for informational subcommands
			return nil
		default:
			if err := configureDevices(c); err != nil {
				log.Fatal(err)
			}
		}

		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:      `get`,
			Usage:     `Retrieve one or more values (using the "QSTN" protocol command)`,
			ArgsUsage: `COMMAND`,
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  `only-value, o`,
					Usage: `Only print the returned value (and exit non-zero if it's empty)`,
				},
			},
			Action: func(c *cli.Context) {
				if code := c.Args().First(); code != `` {
					if err := device.Send(code, `QSTN`); err == nil {
						select {
						case message := <-device.Messages():
							if ci, value, err := MessageToCommand(`QSTN`, message); err == nil {
								v := ``

								if value != nil {
									v = value.String()
								}

								if c.Bool(`only-value`) {
									if v != `` {
										fmt.Println(v)
									}
								} else {
									fmt.Printf("%s\t%s\t%s\t%s\n", ci.Code, v, ci.Name, ci.Description)
								}

								if v == `` {
									os.Exit(1)
								}
							} else {
								log.Fatal(err)
							}
						case <-time.After(c.GlobalDuration(`response-timeout`)):
						}
					} else {
						log.Fatalf("Failed to send command: %v", err)
					}
				} else {
					log.Fatalf("Must specify a command area to query.")
				}
			},
		}, {
			Name:      `call`,
			Usage:     `Execute a given command.`,
			ArgsUsage: `COMMAND SUBCOMMAND [ARGS]`,
			Action: func(c *cli.Context) {
				if code := c.Args().First(); code != `` {
					subcommand := c.Args().Get(2)

					if err := device.Send(code, c.Args().Tail()...); err == nil {
						select {
						case message := <-device.Messages():
							if _, _, err := MessageToCommand(subcommand, message); err != nil {
								log.Fatal(err)
							}
						case <-time.After(c.GlobalDuration(`response-timeout`)):
						}
					} else {
						log.Fatalf("Failed to send command: %v", err)
					}
				} else {
					log.Fatalf("Must specify a command area to query.")
				}
			},
		}, {
			Name:  `serve`,
			Usage: `Connect to a device and continuously monitor events.`,
			Action: func(c *cli.Context) {
				queries := make(chan []string)

				go func() {
					for {
						select {
						case query := <-queries:
							if len(query) >= 2 {
								if err := device.Send(query[0], query[1:]...); err == nil {
									select {
									case message := <-device.Messages():
										if _, _, err := MessageToCommand(query[1], message); err != nil {
											log.Error(err)
										}
									case <-time.After(c.GlobalDuration(`response-timeout`)):
										log.Errorf("Timed out waiting for reply")
									}
								} else {
									log.Errorf("Failed to send command: %v", err)
								}
							} else {
								log.Warningf("Invalid syntax: commands must be in the format: COMMAND SUBCOMMAND [ARGS ..]")
							}

						case message := <-device.Messages():
							if ci, _, err := MessageToCommand(message.Code(), message); err == nil {
								fmt.Printf("%d\t%s\n", int(time.Now().UnixNano()/1000000), ci.String())
							} else {
								log.Errorf("Message Error: %v", err)
							}
						}
					}
				}()

				scanner := bufio.NewScanner(os.Stdin)

				for scanner.Scan() {
					line := scanner.Text()
					queries <- strings.Split(line, ` `)
				}
			},
		}, {
			Name:      `help`,
			Usage:     `Show the documentation for a given command`,
			ArgsUsage: `COMMAND`,
			Action: func(c *cli.Context) {
				commandName := c.Args().First()
				commands := make([]*CommandInfo, 0)

				if commandName != `` {
					if cmd, ok := codeToCmd[c.Args().First()]; ok && cmd != nil {
						commands = append(commands, cmd)
					} else {
						log.Fatalf("Could not find information on command %q", c.Args().First())
					}
				} else {
					keys := make([]string, 0)

					for name, _ := range codeToCmd {
						keys = append(keys, name)
					}

					sort.Strings(keys)

					for _, key := range keys {
						if cmd, ok := codeToCmd[key]; ok && cmd != nil {
							commands = append(commands, cmd)
						}
					}
				}

				for _, cmd := range commands {
					fmt.Printf("%s - %s (zone: %s)\n", cmd.Code, cmd.Description, cmd.Zone)

					if len(cmd.Values) > 0 {
						fmt.Printf("\nSubcommands:\n")

						for _, subcommand := range cmd.Values {
							fmt.Printf("  %-10s %s\n", subcommand.Code, strings.Replace(subcommand.Description, "\n", "\n    ", -1))
						}

						fmt.Printf("\n")
					}
				}
			},
		},
	}

	app.Run(os.Args)
}
