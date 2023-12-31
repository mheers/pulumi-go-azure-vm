package main

import (
	"encoding/base64"

	"github.com/pulumi/pulumi-azure-native-sdk/compute/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/network/v2"
	"github.com/pulumi/pulumi-azure-native-sdk/resources/v2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Create an Azure Resource Group
		resourceGroup, err := resources.NewResourceGroup(ctx, "resourceGroup", nil)
		if err != nil {
			return err
		}

		// Create an Azure Virtual Network
		virtualNetwork, err := network.NewVirtualNetwork(ctx, "virtualNetwork", &network.VirtualNetworkArgs{
			ResourceGroupName: resourceGroup.Name,
			AddressSpace: &network.AddressSpaceArgs{
				AddressPrefixes: pulumi.StringArray{
					pulumi.String("10.0.0.0/16"),
				},
			},
		})
		if err != nil {
			return err
		}

		nsg, err := network.NewNetworkSecurityGroup(ctx, "nsg", &network.NetworkSecurityGroupArgs{
			ResourceGroupName: resourceGroup.Name,

			SecurityRules: network.SecurityRuleTypeArray{
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("allow-http-inbound"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRange:     pulumi.String("80"),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("allow-http-inbound"),
					Priority:                 pulumi.Int(100),
					Protocol:                 pulumi.String("TCP"),
					SourceAddressPrefix:      pulumi.String(""),
					SourceAddressPrefixes: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					SourcePortRange: pulumi.String("*"),
					Type:            pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("allow-ssh-inbound"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRange:     pulumi.String("1988"),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("allow-ssh-inbound"),
					Priority:                 pulumi.Int(110),
					Protocol:                 pulumi.String("TCP"),
					SourceAddressPrefix:      pulumi.String(""),
					SourceAddressPrefixes: pulumi.StringArray{
						pulumi.String("0.0.0.0/0"),
					},
					SourcePortRange: pulumi.String("*"),
					Type:            pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},
			},
		},
			pulumi.DependsOn([]pulumi.Resource{resourceGroup}),
		)
		if err != nil {
			return err
		}

		// Create an Azure Subnet
		subnet, err := network.NewSubnet(ctx, "subnet", &network.SubnetArgs{
			ResourceGroupName:  resourceGroup.Name,
			VirtualNetworkName: virtualNetwork.Name,
			AddressPrefix:      pulumi.String("10.0.1.0/24"),
			NetworkSecurityGroup: network.NetworkSecurityGroupTypeArgs{
				Id: nsg.ID(),
			},
		})
		if err != nil {
			return err
		}

		// Create an Azure Public IP Address
		publicIp, err := network.NewPublicIPAddress(ctx, "publicIP", &network.PublicIPAddressArgs{
			ResourceGroupName:        resourceGroup.Name,
			PublicIpAddressName:      pulumi.String("publicip"),
			PublicIPAllocationMethod: pulumi.String("Static"),
		})
		if err != nil {
			return err
		}

		// Create a Network Interface and assign the public IP
		nic, err := network.NewNetworkInterface(ctx, "networkInterface", &network.NetworkInterfaceArgs{
			ResourceGroupName: resourceGroup.Name,
			IpConfigurations: network.NetworkInterfaceIPConfigurationArray{
				&network.NetworkInterfaceIPConfigurationArgs{
					Name: pulumi.String("webserveripcfg"),
					Subnet: network.SubnetTypeArgs{
						Id: subnet.ID(),
					},
					PrivateIPAllocationMethod: pulumi.String("Dynamic"),
					PublicIPAddress: network.PublicIPAddressTypeArgs{
						Id: publicIp.ID(),
					},
				},
			},
			NetworkSecurityGroup: network.NetworkSecurityGroupTypeArgs{
				Id: nsg.ID(),
			},
		})
		if err != nil {
			return err
		}

		vm, err := compute.NewVirtualMachine(ctx, "my-vm", &compute.VirtualMachineArgs{
			ResourceGroupName: resourceGroup.Name,
			OsProfile: &compute.OSProfileArgs{
				AdminUsername: pulumi.String("testadmin"),
				AdminPassword: pulumi.String(""), // TODO: add your password here
				LinuxConfiguration: compute.LinuxConfigurationArgs{
					Ssh: compute.SshConfigurationArgs{
						PublicKeys: compute.SshPublicKeyTypeArray{
							compute.SshPublicKeyTypeArgs{
								KeyData: pulumi.String(""), // TODO: add your ssh public key here
								Path:    pulumi.String("/home/testadmin/.ssh/authorized_keys"),
							},
						},
					},
					PatchSettings: compute.LinuxPatchSettingsArgs{
						AutomaticByPlatformSettings: compute.LinuxVMGuestPatchAutomaticByPlatformSettingsArgs{
							BypassPlatformSafetyChecksOnUserSchedule: pulumi.Bool(false),
							RebootSetting:                            pulumi.String("IfRequired"),
						},
						PatchMode: pulumi.String("AutomaticByPlatform"),
					},
					EnableVMAgentPlatformUpdates:  pulumi.Bool(true),
					DisablePasswordAuthentication: pulumi.Bool(false),
					ProvisionVMAgent:              pulumi.Bool(true),
				},
				ComputerName: pulumi.String("my-pulumi-vm"),
				CustomData: pulumi.String(base64.StdEncoding.EncodeToString([]byte(`#!/bin/bash
echo "Hello, World!" > index.html
nohup python3 -m http.server 80 &
echo "Port 1988" >> /etc/ssh/sshd_config
systemctl restart sshd
`))),
			},
			StorageProfile: compute.StorageProfileArgs{
				ImageReference: compute.ImageReferenceArgs{
					// az vm image list --output table
					Offer:     pulumi.String("0001-com-ubuntu-server-jammy"),
					Publisher: pulumi.String("Canonical"),
					Sku:       pulumi.String("22_04-lts-gen2"),
					Version:   pulumi.String("latest"),
				},
				OsDisk: compute.OSDiskArgs{
					CreateOption: pulumi.String("FromImage"),
					DeleteOption: pulumi.String("Delete"),
				},
			},
			HardwareProfile: compute.HardwareProfileArgs{
				// see https://learn.microsoft.com/en-us/azure/virtual-machines/av2-series
				// az vm list-skus --location westeurope --size Standard_A --all --output table
				// az vm list-skus --location westeurope --size Standard_B --all --output table
				// Tests:
				// - VirtualMachineSizeTypes_Standard_A2_v2: cannot boot Hypervisor Generation '2'.
				// - VirtualMachineSizeTypes_Standard_A2: Not available in westeurope.
				VmSize:           pulumi.String(compute.VirtualMachineSizeTypes_Standard_B1s),
				VmSizeProperties: compute.VMSizePropertiesArgs{},
			},
			NetworkProfile: compute.NetworkProfileArgs{
				NetworkInterfaces: compute.NetworkInterfaceReferenceArray{
					compute.NetworkInterfaceReferenceArgs{
						Id: nic.ID(),
					},
				},
			},
		},
			pulumi.DependsOn([]pulumi.Resource{resourceGroup, nic}),
		)
		if err != nil {
			return err
		}

		ctx.Export("vm Id", vm.ID())
		ctx.Export("vm IP public", publicIp.IpAddress)
		ctx.Export("vm IP private", nic.IpConfigurations.Index(pulumi.Int(0)).PrivateIPAddress())

		// Create an Azure Network Security Group for Bastion host
		nsgBastion, err := network.NewNetworkSecurityGroup(ctx, "nsgBastion", &network.NetworkSecurityGroupArgs{
			ResourceGroupName: resourceGroup.Name,
			SecurityRules: network.SecurityRuleTypeArray{
				// ingress
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("Default SCF Rule deny hubspace inbound"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRange:     pulumi.String("443"),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("AllowHttpsInbound"),
					Priority:                 pulumi.Int(120),
					Protocol:                 pulumi.String("TCP"),
					SourceAddressPrefix:      pulumi.String("Internet"),
					SourcePortRange:          pulumi.String("*"),
					Type:                     pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("Default SCF Rule deny hubspace inbound"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRange:     pulumi.String("443"),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("AllowGatewayManagerInbound"),
					Priority:                 pulumi.Int(130),
					Protocol:                 pulumi.String("TCP"),
					SourceAddressPrefix:      pulumi.String("GatewayManager"),
					SourcePortRange:          pulumi.String("*"),
					Type:                     pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("Default SCF Rule deny hubspace inbound"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRange:     pulumi.String("443"),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("AllowAzureLoadBalancerInbound"),
					Priority:                 pulumi.Int(140),
					Protocol:                 pulumi.String("TCP"),
					SourceAddressPrefix:      pulumi.String("AzureLoadBalancer"),
					SourcePortRange:          pulumi.String("*"),
					Type:                     pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("Default SCF Rule deny hubspace inbound"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRanges:    pulumi.ToStringArray([]string{"8080", "5701"}),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("AllowBastionHostCommunication"),
					Priority:                 pulumi.Int(150),
					Protocol:                 pulumi.String("*"),
					SourceAddressPrefix:      pulumi.String("VirtualNetwork"),
					SourcePortRange:          pulumi.String("*"),
					Type:                     pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},

				// egress
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("Default SCF Rule deny hubspace inbound"),
					DestinationAddressPrefix: pulumi.String("VirtualNetwork"),
					DestinationPortRanges:    pulumi.ToStringArray([]string{"22", "3389"}),
					Direction:                pulumi.String("Outbound"),
					Name:                     pulumi.String("AllowSshRdpOutbound"),
					Priority:                 pulumi.Int(100),
					Protocol:                 pulumi.String("*"),
					SourceAddressPrefix:      pulumi.String("*"),
					SourcePortRange:          pulumi.String("*"),
					Type:                     pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("Default SCF Rule deny hubspace inbound"),
					DestinationAddressPrefix: pulumi.String("AzureCloud"),
					DestinationPortRange:     pulumi.String("443"),
					Direction:                pulumi.String("Outbound"),
					Name:                     pulumi.String("AllowAzureCloudOutbound"),
					Priority:                 pulumi.Int(110),
					Protocol:                 pulumi.String("TCP"),
					SourceAddressPrefix:      pulumi.String("*"),
					SourcePortRange:          pulumi.String("*"),
					Type:                     pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("Default SCF Rule deny hubspace inbound"),
					DestinationAddressPrefix: pulumi.String("VirtualNetwork"),
					DestinationPortRanges:    pulumi.ToStringArray([]string{"8080", "5701"}),
					Direction:                pulumi.String("Outbound"),
					Name:                     pulumi.String("AllowBastionCommunication"),
					Priority:                 pulumi.Int(120),
					Protocol:                 pulumi.String("*"),
					SourceAddressPrefix:      pulumi.String("VirtualNetwork"),
					SourcePortRange:          pulumi.String("*"),
					Type:                     pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},
				&network.SecurityRuleTypeArgs{
					Access:                   pulumi.String("Allow"),
					Description:              pulumi.String("Default SCF Rule deny hubspace inbound"),
					DestinationAddressPrefix: pulumi.String("Internet"),
					DestinationPortRange:     pulumi.String("80"),
					Direction:                pulumi.String("Outbound"),
					Name:                     pulumi.String("AllowHttpOutbound"),
					Priority:                 pulumi.Int(130),
					Protocol:                 pulumi.String("*"),
					SourceAddressPrefix:      pulumi.String("*"),
					SourcePortRange:          pulumi.String("*"),
					Type:                     pulumi.String("Microsoft.Network/networkSecurityGroups/securityRules"),
				},
			},
		},
			pulumi.DependsOn([]pulumi.Resource{resourceGroup}),
		)
		if err != nil {
			return err
		}

		// Create an Azure Subnet for Bastion host
		subnetBastion, err := network.NewSubnet(ctx, "subnetBastion", &network.SubnetArgs{
			SubnetName:         pulumi.String("AzureBastionSubnet"),
			ResourceGroupName:  resourceGroup.Name,
			VirtualNetworkName: virtualNetwork.Name,
			AddressPrefix:      pulumi.String("10.0.2.0/26"),
			NetworkSecurityGroup: network.NetworkSecurityGroupTypeArgs{
				Id: nsgBastion.ID(),
			},
		})
		if err != nil {
			return err
		}

		// Create an Azure Public IP Address for Bastion host
		publicIpBastion, err := network.NewPublicIPAddress(ctx, "publicIPBastion", &network.PublicIPAddressArgs{
			ResourceGroupName:        resourceGroup.Name,
			PublicIpAddressName:      pulumi.String("publicipBastion"),
			PublicIPAllocationMethod: pulumi.String("Static"),
			Sku: network.PublicIPAddressSkuArgs{
				Name: pulumi.String("Standard"),
				Tier: pulumi.String("Regional"),
			},
		})
		if err != nil {
			return err
		}

		// Create a Bastion host
		bastion, err := network.NewBastionHost(ctx, "bastionHost", &network.BastionHostArgs{
			ResourceGroupName: resourceGroup.Name,
			BastionHostName:   pulumi.String("bastionhosttenant"),
			Sku: network.SkuArgs{
				Name: pulumi.String("Basic"),
			},
			IpConfigurations: network.BastionHostIPConfigurationArray{
				network.BastionHostIPConfigurationArgs{
					Name: pulumi.String("bastionHostIpConfiguration"),
					Subnet: network.SubResourceArgs{
						Id: subnetBastion.ID(),
					},
					PublicIPAddress: network.SubResourceArgs{
						Id: publicIpBastion.ID(),
					},
				},
			},
		})
		if err != nil {
			return err
		}

		ctx.Export("bastion", bastion.Name)
		ctx.Export("bastion IP public", publicIpBastion.IpAddress)

		return nil
	})
}
