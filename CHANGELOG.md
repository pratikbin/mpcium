# CHANGELOG

## v0.2.0 (2025-04-12)

- Use onSuccess callback when sign session succeeds [View](https://github.com/fystack/mpcium/commit/9602d4d9bfe37c2d038856d3ed206bfecd2e8c93)
- Fix bug signing doesn't work after all nodes are backup [View](https://github.com/fystack/mpcium/commit/a9192ca11581dd986bdd21728cbda4b78d75a753)
- Handle duplicate message [View](https://github.com/fystack/mpcium/commit/e79f6e20fbe225e5aad8b0c9e70578356fce9573)
- Update timeout consumer keep subscribe on time [View](https://github.com/fystack/mpcium/commit/52ee83c3ecc2bbb8c16a8227f4f00b72a57c8499)
- Update signing timeout logic when not enough participants [View](https://github.com/fystack/mpcium/commit/e8ffa381f489a83e60dbcbf5262927e99eca2382)
- Persit message, handle failure and timeout sign tx [View](https://github.com/fystack/mpcium/commit/400f26912ea6b31cbf511de93c1270776055c758)

## v0.1.7 (2024-10-05)

- Fix bug sign mutiple transactions simulteneously crash if transactions have the same walletID [View](https://github.com/fystack/mpcium/commit/7163097387ca2c682f49bab3e4bd8ad58b33ff29)
- Revert "Debug" [View](https://github.com/fystack/mpcium/commit/f75077717d7cce0b9cfacb349470269ff1ec427b)
- Fix don't pass walletId to session [View](https://github.com/fystack/mpcium/commit/1d505fc8b5724daaf9bbdfa26380ee371ddd95ca)
- Debug [View](https://github.com/fystack/mpcium/commit/4810e7f553939821cce33859d871228eeebe0a2b)
- Fix topic composer logic [View](https://github.com/fystack/mpcium/commit/203cc7707486c2089a6af61f727fb60962b84b88)
- Update mask logic [View](https://github.com/fystack/mpcium/commit/6a3869489ef4881617839fdc1e062582293fc64e)
- Update topic composer [View](https://github.com/fystack/mpcium/commit/d06f23f16d9cd47823d1b5a5cae3428ec7bb5617)
- Minor fix on make file [View](https://github.com/fystack/mpcium/commit/f376c4f8d170ff49c8eb07b4df8d76a38f2134ae)
- Fix bug, load nats url from env var [View](https://github.com/fystack/mpcium/commit/ccfb81159a07f497dc2d8340a3b0b763719e5bcb)
- Update changelog v0.1.6 [View](https://github.com/fystack/mpcium/commit/87220e795b44ecc14759e71c5224c62b3bec08d3)

## v0.1.6 (2024-09-22)

- Add deployment script mpcium [View](https://github.com/fystack/mpcium/commit/5b97ce9b1208eafdf104f4045d6614889d470b93)
- Log config in generateid script [View](https://github.com/fystack/mpcium/commit/8f244d313655e5eb8a73b55cc5b3488e36d86488)
- Update configuration load logic, mask sensitive data [View](https://github.com/fystack/mpcium/commit/974b7c404a369fa6211386780881cb407ee8eee2)
- Update makefile [View](https://github.com/fystack/mpcium/commit/affc8300bf4c4cb18af0cc6a347fc8f1d8e9565a)
- Fix load config, it doens't load environment variables into struct [View](https://github.com/fystack/mpcium/commit/744e8ce48dde25fce460eaad4d73ebf2f38e2be3)
- Implement connection for prod [View](https://github.com/fystack/mpcium/commit/5e10a81f12c0879fe8ffd1ee918068ebc50114d3)
- Update changelog [View](https://github.com/fystack/mpcium/commit/d0572d20839b2b512da04f63ffc2d1fc28610cbf)

## v0.1.5 (2024-06-22)

- Add a slight delay before start sending key generation messages [View](https://github.com/fystack/mpcium/commit/c8229c0a32510eb3faeb7dc2025b4832cb65c715)
- Add type to sessions [View](https://github.com/fystack/mpcium/commit/c7c70e36c39125e6899f5e315493a0a84e47f2dd)
- Add comment to add distributed lock [View](https://github.com/fystack/mpcium/commit/66d106838f335eae3852d4434f87d2cdf9efe6dd)
- Add delay temporary solution to fix the issue that sign before all nodes are ready [View](https://github.com/fystack/mpcium/commit/f30c1c5c543e360f5691fe3e434a75fcefe83056)

## v0.1.4 (2024-05-19)

- Add script to add prefix ecdsa for existing keyinfos [View](https://github.com/fystack/mpcium/commit/9495ce20aea153ba00abde65a6628bf1f2602144)
- Return only Signature in siging success event [View](https://github.com/fystack/mpcium/commit/c915a5f7f925b85af67f0a5dd6c5ba29f3eee818)
- Support eddsa signing [View](https://github.com/fystack/mpcium/commit/efc5125ed60ca774c382a5bcb01bb6de6fa548f0)
- Add eddsa signing session [View](https://github.com/fystack/mpcium/commit/fc9f2c20bb41c8542edc3bbecaf261c44a52122d)
- Init logger for migration script [View](https://github.com/fystack/mpcium/commit/9159523bade220ca83fd6db81b17ccf600c9229c)

## v0.1.3 (2024-05-11)

- Add script to add prefix ecdsa for old keys [View](https://github.com/fystack/mpcium/commit/934c37c9dc6b68c2ab5a7a7afe71758aae9f44ed)
- Include eddsa pubkey in SucessGenerationEvent [View](https://github.com/fystack/mpcium/commit/92a102ed2d037e2b6929d70d082795454db42f14)
- Gen eddsa key works now [View](https://github.com/fystack/mpcium/commit/44866f36d37bfa16bcfc610e057fd7a3037e9efb)
- Support EDDSA [View](https://github.com/fystack/mpcium/commit/722e636ad00edd57243b62e877a01f629aa27b84)
- Update changelog [View](https://github.com/fystack/mpcium/commit/e9ed0f67279b6476c8d1a8638b0baad59ebd018c)

## v0.1.2 (2024-03-09)

- Change default threshhold to 1 [View](https://github.com/fystack/mpcium/commit/f360810aa760b52871a7cba0b107d09e6bbd7d47)
- Refine log [View](https://github.com/fystack/mpcium/commit/a9cde4014c16cfe3ca6de73039b8e13c49bb65d0)
- Threshold validation, allow t+1 peers to create signing session [View](https://github.com/fystack/mpcium/commit/b5c15463fa01f58ce9557ebffaa0e96ce6dcda2d)
- Implement keyinfo store [View](https://github.com/fystack/mpcium/commit/d39168ddd7fc622eb1a58ea90617680249297515)
- Track ready nodes, reduce readiness period to 1 second [View](https://github.com/fystack/mpcium/commit/75cb2b286fffb89f1df39c680dee3d1cacfcffc9)
- Upgrade tss-lib to v2 [View](https://github.com/fystack/mpcium/commit/cd324358d7c297d2025ea2d0c02464b5552f513d)

## v0.1.1 (2024-02-05)

- Add retry package to improve NATs direct message resiliency [View](https://github.com/fystack/mpcium/commit/195f9a4c50732919994b67c13396f141fa4efcdf)
- Move listening to incoming message prior to genkey and signing to fix the nats: no responders available for request [View](https://github.com/fystack/mpcium/commit/421d02e947d12324c188d9bb2868cfb9ee02c3ca)
- Increase direct messsage timeout, add mutex lock for party updating [View](https://github.com/fystack/mpcium/commit/1d75eeea669212ff4b3168575cc07d4e8a0280ae)
- Pass done function to session to clean up resource after successful execution [View](https://github.com/fystack/mpcium/commit/e5430315a3ddc4a0b74b456ee9f4f68b185b5c5e)
- Implement queue manager to spawn message queue based on topic [View](https://github.com/fystack/mpcium/commit/7c107c9cba7db68358df77ea00b00d8d6b659d1d)
- Decode round8, round9 [View](https://github.com/fystack/mpcium/commit/ccd869e8d827ecfeae453b0f865899d35b520e0f)
- Refactor signing session [View](https://github.com/fystack/mpcium/commit/4447cc46da86c03a2353edc7f01b85fd79ead084)
- Clean up code [View](https://github.com/fystack/mpcium/commit/b40d8a42082ed00099054b9852a3415ca24426d0)
- Refactor keygen session [View](https://github.com/fystack/mpcium/commit/11bc34bb5831b0b3ad39fe14e77ef56a931d023d)
- Fix script to load peer ids to consul [View](https://github.com/fystack/mpcium/commit/70a1b53c350ce6414cca308aa588a53495c9411f)
