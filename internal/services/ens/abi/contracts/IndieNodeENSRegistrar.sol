// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@ensdomains/ens-contracts/contracts/registry/ENSRegistry.sol";
import "@ensdomains/ens-contracts/contracts/ethregistrar/ETHRegistrarController.sol";
import "@ensdomains/ens-contracts/contracts/ethregistrar/IPriceOracle.sol";

contract IndieNodeENSRegistrar is Ownable, ReentrancyGuard {
    ETHRegistrarController public immutable controller;
    uint256 public FEE = 0.001 ether; // 0.001 ETH fee

    constructor(address _controller) {
        require(_controller != address(0), "Invalid controller address");
        controller = ETHRegistrarController(_controller);
    }

    // Function to register an ENS name with a fee
    function registerWithFee(
        string calldata name,
        address owner,
        uint256 duration,
        bytes32 secret,
        address resolver,
        bytes[] calldata data,
        bool reverseRecord,
        uint16 ownerControlledFuses
    ) external payable nonReentrant {
        // Calculate total cost including our fee
        IPriceOracle.Price memory price = controller.rentPrice(name, duration);
        uint256 totalCost = price.base + price.premium + FEE;

        // Verify sufficient payment
        require(msg.value >= totalCost, "Insufficient payment");

        // Forward registration to ENS controller
        controller.register{value: price.base + price.premium}(
            name,
            owner,
            duration,
            secret,
            resolver,
            data,
            reverseRecord,
            ownerControlledFuses
        );

        // Keep the fee in the contract
        // The fee will be available for withdrawal by the owner
    }

    // Function to withdraw collected fees
    function withdraw() external onlyOwner {
        uint256 balance = address(this).balance;
        require(balance > 0, "No fees to withdraw");
        
        (bool success, ) = owner().call{value: balance}("");
        require(success, "Withdrawal failed");
    }

    // Function to update the fee (optional)
    function updateFee(uint256 newFee) external onlyOwner {
        require(newFee <= 0.01 ether, "Fee too high"); // Cap at 0.01 ETH
        FEE = newFee;
    }
}
