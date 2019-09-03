pragma solidity ^0.5.10;

import "@openzeppelin/contracts/GSN/GSNRecipient.sol";

contract MockRecipient is GSNRecipient {
    event SampleRecipientEmitted(string message, address realSender, address msgSender, address origin);
    event SampleRecipientPreCall();
    event SampleRecipientPostCall(bool success, uint256 actualCharge, bytes32 preRetVal);

    function emitMessage(string memory message) public {
        emit SampleRecipientEmitted(message, _msgSender(), msg.sender, tx.origin);
    }

    function dontEmitMessage(string memory message) public {}

    function acceptRelayedCall(address, address, bytes calldata, uint256, uint256, uint256, uint256, bytes calldata, uint256) external view returns (uint256, bytes memory) {
        return _approveRelayedCall();
    }

    function _preRelayedCall(bytes memory) internal returns (bytes32) {
        emit SampleRecipientPreCall();
        return bytes32(uint256(123456));
    }

    function _postRelayedCall(bytes memory, bool success, uint256 actualCharge, bytes32 preRetVal) internal {
        emit SampleRecipientPostCall(success, actualCharge, preRetVal);
    }
}

