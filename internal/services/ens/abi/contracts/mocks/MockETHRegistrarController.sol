// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@ensdomains/ens-contracts/contracts/ethregistrar/IETHRegistrarController.sol";
import "@ensdomains/ens-contracts/contracts/ethregistrar/IPriceOracle.sol";

contract MockETHRegistrarController is IETHRegistrarController {
    uint256 private _basePrice;
    uint256 private _premiumPrice;
    string private _lastRegisteredName;
    bool private _available;

    constructor() {
        _available = true;
    }

    function setPrice(uint256 base, uint256 premium) external {
        _basePrice = base;
        _premiumPrice = premium;
    }

    function setAvailable(bool available) external {
        _available = available;
    }

    function rentPrice(
        string memory,
        uint256
    ) external view override returns (IPriceOracle.Price memory) {
        return IPriceOracle.Price({
            base: _basePrice,
            premium: _premiumPrice
        });
    }

    function available(string memory) external view override returns (bool) {
        return _available;
    }

    function makeCommitment(
        string memory,
        address,
        uint256,
        bytes32,
        address,
        bytes[] calldata,
        bool,
        uint16
    ) external pure override returns (bytes32) {
        return bytes32(0);
    }

    function commit(bytes32) external override {}

    function register(
        string calldata name,
        address,
        uint256,
        bytes32,
        address,
        bytes[] calldata,
        bool,
        uint16
    ) external payable override {
        _lastRegisteredName = name;
    }

    function renew(string calldata, uint256) external payable override {}

    function lastRegisteredName() external view returns (string memory) {
        return _lastRegisteredName;
    }
} 