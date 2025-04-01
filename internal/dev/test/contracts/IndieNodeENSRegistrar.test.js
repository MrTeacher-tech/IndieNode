const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("IndieNodeENSRegistrar", function () {
    let IndieNodeENSRegistrar;
    let registrar;
    let mockController;
    let owner;
    let user;
    let addrs;

    beforeEach(async function () {
        // Get test accounts
        [owner, user, ...addrs] = await ethers.getSigners();

        // Deploy a mock controller first
        const MockController = await ethers.getContractFactory("MockETHRegistrarController");
        mockController = await MockController.deploy();
        await mockController.deployed();

        // Deploy the registrar contract
        IndieNodeENSRegistrar = await ethers.getContractFactory("IndieNodeENSRegistrar");
        registrar = await IndieNodeENSRegistrar.deploy(mockController.address);
        await registrar.deployed();
    });

    describe("Initialization", function () {
        it("Should set the correct owner", async function () {
            expect(await registrar.owner()).to.equal(owner.address);
        });

        it("Should set the correct initial fee", async function () {
            expect(await registrar.FEE()).to.equal(ethers.utils.parseEther("0.001"));
        });

        it("Should set the correct controller address", async function () {
            expect(await registrar.controller()).to.equal(mockController.address);
        });
    });

    describe("Fee Management", function () {
        it("Should allow owner to update fee", async function () {
            const newFee = ethers.utils.parseEther("0.005");
            await registrar.updateFee(newFee);
            expect(await registrar.FEE()).to.equal(newFee);
        });

        it("Should revert if non-owner tries to update fee", async function () {
            const newFee = ethers.utils.parseEther("0.005");
            await expect(
                registrar.connect(user).updateFee(newFee)
            ).to.be.revertedWith("Ownable: caller is not the owner");
        });

        it("Should revert if fee is above maximum", async function () {
            const tooHighFee = ethers.utils.parseEther("0.02");
            await expect(
                registrar.updateFee(tooHighFee)
            ).to.be.revertedWith("Fee too high");
        });
    });

    describe("Registration", function () {
        const testName = "test";
        const duration = 31536000; // 1 year in seconds
        const secret = ethers.utils.formatBytes32String("secret");
        const resolver = ethers.constants.AddressZero;
        const data = [];
        const reverseRecord = false;
        const ownerControlledFuses = 0;

        beforeEach(async function () {
            // Mock the rentPrice function to return known values
            await mockController.setPrice(
                ethers.utils.parseEther("0.1"), // base
                ethers.utils.parseEther("0.05")  // premium
            );
        });

        it("Should register name with correct payment", async function () {
            const base = ethers.utils.parseEther("0.1");
            const premium = ethers.utils.parseEther("0.05");
            const fee = await registrar.FEE();
            const totalCost = base.add(premium).add(fee);

            await registrar.connect(user).registerWithFee(
                testName,
                user.address,
                duration,
                secret,
                resolver,
                data,
                reverseRecord,
                ownerControlledFuses,
                { value: totalCost }
            );

            // Verify the registration was called on the controller
            expect(await mockController.lastRegisteredName()).to.equal(testName);
        });

        it("Should revert if payment is insufficient", async function () {
            const insufficientPayment = ethers.utils.parseEther("0.1");
            
            await expect(
                registrar.connect(user).registerWithFee(
                    testName,
                    user.address,
                    duration,
                    secret,
                    resolver,
                    data,
                    reverseRecord,
                    ownerControlledFuses,
                    { value: insufficientPayment }
                )
            ).to.be.revertedWith("Insufficient payment");
        });
    });

    describe("Withdrawal", function () {
        it("Should allow owner to withdraw fees", async function () {
            // First register a name to collect some fees
            const payment = ethers.utils.parseEther("1");
            await registrar.connect(user).registerWithFee(
                "test",
                user.address,
                31536000,
                ethers.utils.formatBytes32String("secret"),
                ethers.constants.AddressZero,
                [],
                false,
                0,
                { value: payment }
            );

            const initialBalance = await owner.getBalance();
            await registrar.withdraw();
            const finalBalance = await owner.getBalance();

            expect(finalBalance.gt(initialBalance)).to.be.true;
        });

        it("Should revert if non-owner tries to withdraw", async function () {
            await expect(
                registrar.connect(user).withdraw()
            ).to.be.revertedWith("Ownable: caller is not the owner");
        });

        it("Should revert if there are no fees to withdraw", async function () {
            await expect(
                registrar.withdraw()
            ).to.be.revertedWith("No fees to withdraw");
        });
    });
});