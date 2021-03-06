package reward

import (
	"bytes"

	"github.com/fletaio/common"
	"github.com/fletaio/common/util"
	"github.com/fletaio/core/amount"
	"github.com/fletaio/core/consensus"
	"github.com/fletaio/core/data"
)

type TestNetRewarder struct {
	LastPaidHeight  uint32
	PowerMap        map[common.Address]*amount.Amount
	StakingPowerMap map[common.Address]map[common.Address]*amount.Amount
}

func NewTestNetRewarder() *TestNetRewarder {
	rd := &TestNetRewarder{
		PowerMap:        map[common.Address]*amount.Amount{},
		StakingPowerMap: map[common.Address]map[common.Address]*amount.Amount{},
	}
	return rd
}

// ApplyGenesis init genesis data
func (rd *TestNetRewarder) ApplyGenesis(ctx *data.ContextData) ([]byte, error) {
	SaveData, err := rd.buildSaveData()
	if err != nil {
		return nil, err
	}
	return SaveData, nil
}

// ProcessReward gives a reward to the block generator address
func (rd *TestNetRewarder) ProcessReward(addr common.Address, ctx *data.Context) ([]byte, error) {
	policy, err := consensus.GetConsensusPolicy(ctx.ChainCoord())
	if err != nil {
		return nil, err
	}

	if true {
		acc, err := ctx.Account(addr)
		if err != nil {
			return nil, err
		}

		frAcc, is := acc.(*consensus.FormulationAccount)
		if !is {
			return nil, consensus.ErrInvalidAccountType
		}
		switch frAcc.FormulationType {
		case consensus.AlphaFormulatorType:
			rd.addRewardPower(addr, frAcc.Amount.MulC(int64(policy.AlphaEfficiency1000)).DivC(1000))
		case consensus.SigmaFormulatorType:
			rd.addRewardPower(addr, frAcc.Amount.MulC(int64(policy.SigmaEfficiency1000)).DivC(1000))
		case consensus.OmegaFormulatorType:
			rd.addRewardPower(addr, frAcc.Amount.MulC(int64(policy.OmegaEfficiency1000)).DivC(1000))
		case consensus.HyperFormulatorType:
			PowerSum := frAcc.Amount.MulC(int64(policy.HyperEfficiency1000)).DivC(1000)

			keys, err := ctx.AccountDataKeys(addr, consensus.TagStaking)
			if err != nil {
				return nil, err
			}
			for _, k := range keys {
				if StakingAddress, is := consensus.FromStakingKey(k); is {
					bs := ctx.AccountData(addr, k)
					if len(bs) == 0 {
						return nil, consensus.ErrInvalidStakingAddress
					}
					StakingAmount := amount.NewAmountFromBytes(bs)

					if _, err := ctx.Account(StakingAddress); err != nil {
						if err != data.ErrNotExistAccount {
							return nil, err
						}
						rd.removeRewardPower(StakingAddress)
					} else {
						StakingPower := StakingAmount.MulC(int64(policy.StakingEfficiency1000)).DivC(1000)
						ComissionPower := StakingPower.MulC(int64(frAcc.Policy.CommissionRatio1000)).DivC(1000)

						if bs := ctx.AccountData(addr, consensus.ToAutoStakingKey(StakingAddress)); len(bs) > 0 && bs[0] == 1 {
							rd.addStakingPower(addr, StakingAddress, StakingPower.Sub(ComissionPower))
							PowerSum = PowerSum.Add(StakingPower)
						} else {
							rd.addRewardPower(StakingAddress, StakingPower.Sub(ComissionPower))
							PowerSum = PowerSum.Add(ComissionPower)
						}
					}
				}
			}
			rd.addRewardPower(addr, PowerSum)
		default:
			return nil, consensus.ErrInvalidAccountType
		}
	}

	if ctx.TargetHeight() >= rd.LastPaidHeight+policy.PayRewardEveryBlocks {
		TotalPower := amount.NewCoinAmount(0, 0)
		for _, PowerSum := range rd.PowerMap {
			TotalPower = TotalPower.Add(PowerSum)
		}
		TotalReward := policy.RewardPerBlock.MulC(int64(ctx.TargetHeight() - rd.LastPaidHeight))
		Ratio := TotalReward.Mul(amount.COIN).Div(TotalPower)
		for RewardAddress, PowerSum := range rd.PowerMap {
			acc, err := ctx.Account(RewardAddress)
			if err != nil {
				if err != data.ErrNotExistAccount {
					return nil, err
				}
			} else {
				frAcc := acc.(*consensus.FormulationAccount)
				frAcc.AddBalance(PowerSum.Mul(Ratio).Div(amount.COIN))
				//log.Println("AddBalance", frAcc.Address().String(), PowerSum.Mul(Ratio).Div(amount.COIN).String())
			}
			rd.removeRewardPower(RewardAddress)
		}

		for HyperAddress, PowerMap := range rd.StakingPowerMap {
			for StakingAddress, PowerSum := range PowerMap {
				bs := ctx.AccountData(HyperAddress, consensus.ToStakingKey(StakingAddress))
				if len(bs) == 0 {
					return nil, consensus.ErrInvalidStakingAddress
				}
				StakingAmount := amount.NewAmountFromBytes(bs)
				ctx.SetAccountData(HyperAddress, consensus.ToStakingKey(StakingAddress), StakingAmount.Add(PowerSum.Mul(Ratio).Div(amount.COIN)).Bytes())
			}
		}
		rd.StakingPowerMap = map[common.Address]map[common.Address]*amount.Amount{}

		//log.Println("Paid at", ctx.TargetHeight())
		rd.LastPaidHeight = ctx.TargetHeight()
	}
	SaveData, err := rd.buildSaveData()
	if err != nil {
		return nil, err
	}
	return SaveData, nil
}

func (rd *TestNetRewarder) addRewardPower(addr common.Address, Power *amount.Amount) {
	//log.Println("addRewardPower", addr.String(), rd.getRewardPower(addr).Add(Power).String())
	rd.PowerMap[addr] = rd.getRewardPower(addr).Add(Power)
}

func (rd *TestNetRewarder) removeRewardPower(addr common.Address) {
	//log.Println("removeRewardPower", addr.String(), nil)
	delete(rd.PowerMap, addr)
}

func (rd *TestNetRewarder) addStakingPower(addr common.Address, StakingAddress common.Address, Power *amount.Amount) {
	//log.Println("addRewardPower", addr.String(), rd.getRewardPower(addr).Add(Power).String())
	PowerMap, has := rd.StakingPowerMap[addr]
	if !has {
		PowerMap = map[common.Address]*amount.Amount{}
		rd.StakingPowerMap[addr] = PowerMap
	}
	PowerMap[StakingAddress] = rd.getRewardPower(addr).Add(Power)
}

func (rd *TestNetRewarder) getRewardPower(addr common.Address) *amount.Amount {
	if PowerSum, has := rd.PowerMap[addr]; has {
		return PowerSum
	} else {
		return amount.NewCoinAmount(0, 0)
	}
}

func (rd *TestNetRewarder) getStakingPower(addr common.Address, StakingAddress common.Address) *amount.Amount {
	if PowerMap, has := rd.StakingPowerMap[addr]; has {
		if PowerSum, has := PowerMap[StakingAddress]; has {
			return PowerSum
		} else {
			return amount.NewCoinAmount(0, 0)
		}
	} else {
		return amount.NewCoinAmount(0, 0)
	}
}

func (rd *TestNetRewarder) buildSaveData() ([]byte, error) {
	var buffer bytes.Buffer
	if _, err := util.WriteUint32(&buffer, rd.LastPaidHeight); err != nil {
		return nil, err
	}
	if _, err := util.WriteUint32(&buffer, uint32(len(rd.PowerMap))); err != nil {
		return nil, err
	} else {
		for addr, PowerSum := range rd.PowerMap {
			if _, err := addr.WriteTo(&buffer); err != nil {
				return nil, err
			}
			if _, err := PowerSum.WriteTo(&buffer); err != nil {
				return nil, err
			}
		}
	}
	if _, err := util.WriteUint32(&buffer, uint32(len(rd.StakingPowerMap))); err != nil {
		return nil, err
	} else {
		for addr, PowerMap := range rd.StakingPowerMap {
			if _, err := addr.WriteTo(&buffer); err != nil {
				return nil, err
			}
			if _, err := util.WriteUint32(&buffer, uint32(len(PowerMap))); err != nil {
				return nil, err
			} else {
				for StakingAddress, PowerSum := range PowerMap {
					if _, err := StakingAddress.WriteTo(&buffer); err != nil {
						return nil, err
					}
					if _, err := PowerSum.WriteTo(&buffer); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return buffer.Bytes(), nil
}

// LoadFromSaveData recover the status using the save data
func (rd *TestNetRewarder) LoadFromSaveData(SaveData []byte) error {
	r := bytes.NewReader(SaveData)
	if v, _, err := util.ReadUint32(r); err != nil {
		return err
	} else {
		rd.LastPaidHeight = v
	}
	if Len, _, err := util.ReadUint32(r); err != nil {
		return err
	} else {
		rd.PowerMap = map[common.Address]*amount.Amount{}
		for i := 0; i < int(Len); i++ {
			var addr common.Address
			if _, err := addr.ReadFrom(r); err != nil {
				return err
			}
			Amount := amount.NewCoinAmount(0, 0)
			if _, err := Amount.ReadFrom(r); err != nil {
				return err
			}
			rd.PowerMap[addr] = Amount
		}
	}
	if Len, _, err := util.ReadUint32(r); err != nil {
		return err
	} else {
		rd.StakingPowerMap = map[common.Address]map[common.Address]*amount.Amount{}
		for i := 0; i < int(Len); i++ {
			var addr common.Address
			if _, err := addr.ReadFrom(r); err != nil {
				return err
			}
			if Len2, _, err := util.ReadUint32(r); err != nil {
				return err
			} else {
				PowerMap := map[common.Address]*amount.Amount{}
				for j := 0; j < int(Len2); i++ {
					var StakingAddress common.Address
					if _, err := StakingAddress.ReadFrom(r); err != nil {
						return err
					}
					Amount := amount.NewCoinAmount(0, 0)
					if _, err := Amount.ReadFrom(r); err != nil {
						return err
					}
					PowerMap[StakingAddress] = Amount
				}
				rd.StakingPowerMap[addr] = PowerMap
			}
		}
	}
	return nil
}
