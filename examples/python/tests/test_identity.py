"""Identity scenario: STS GetCallerIdentity."""


def test_caller_identity_is_emulator_account(sts):
    identity = sts.get_caller_identity()

    # The emulator always uses this fixed account ID.
    assert identity["Account"] == "000000000000"
