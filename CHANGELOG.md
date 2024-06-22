# CHANGELOG

This changelog is a work in progress and may contain notes for versions which have not actually been released. Check the [Releases](https://github.com/cryptoniumX/mpcium/releases) page to see full release notes and more information about the latest released versions.

## v0.1.5 (2024-06-22)

- Add a slight delay before start sending key generation messages [View](https://github.com/cryptoniumX/mpcium/commit/c8229c0a32510eb3faeb7dc2025b4832cb65c715)
- Add type to sessions [View](https://github.com/cryptoniumX/mpcium/commit/c7c70e36c39125e6899f5e315493a0a84e47f2dd)
- Add comment to add distributed lock [View](https://github.com/cryptoniumX/mpcium/commit/66d106838f335eae3852d4434f87d2cdf9efe6dd)
- Add delay temporary solution to fix the issue that sign before all nodes are ready [View](https://github.com/cryptoniumX/mpcium/commit/f30c1c5c543e360f5691fe3e434a75fcefe83056)

## v0.1.4 (2024-05-19)

- Add script to add prefix ecdsa for existing keyinfos [View](https://github.com/cryptoniumX/mpcium/commit/9495ce20aea153ba00abde65a6628bf1f2602144)
- Return only Signature in siging success event [View](https://github.com/cryptoniumX/mpcium/commit/c915a5f7f925b85af67f0a5dd6c5ba29f3eee818)
- Support eddsa signing [View](https://github.com/cryptoniumX/mpcium/commit/efc5125ed60ca774c382a5bcb01bb6de6fa548f0)
- Add eddsa signing session [View](https://github.com/cryptoniumX/mpcium/commit/fc9f2c20bb41c8542edc3bbecaf261c44a52122d)
- Init logger for migration script [View](https://github.com/cryptoniumX/mpcium/commit/9159523bade220ca83fd6db81b17ccf600c9229c)

## v0.1.3 (2024-05-11)

- Add script to add prefix ecdsa for old keys [View](https://github.com/cryptoniumX/mpcium/commit/934c37c9dc6b68c2ab5a7a7afe71758aae9f44ed)
- Include eddsa pubkey in SucessGenerationEvent [View](https://github.com/cryptoniumX/mpcium/commit/92a102ed2d037e2b6929d70d082795454db42f14)
- Gen eddsa key works now [View](https://github.com/cryptoniumX/mpcium/commit/44866f36d37bfa16bcfc610e057fd7a3037e9efb)
- Support EDDSA [View](https://github.com/cryptoniumX/mpcium/commit/722e636ad00edd57243b62e877a01f629aa27b84)
- Update changelog [View](https://github.com/cryptoniumX/mpcium/commit/e9ed0f67279b6476c8d1a8638b0baad59ebd018c)

## v0.1.2 (2024-03-09)

- Change default threshhold to 1 [View](https://github.com/cryptoniumX/mpcium/commit/f360810aa760b52871a7cba0b107d09e6bbd7d47)
- Refine log [View](https://github.com/cryptoniumX/mpcium/commit/a9cde4014c16cfe3ca6de73039b8e13c49bb65d0)
- Threshold validation, allow t+1 peers to create signing session [View](https://github.com/cryptoniumX/mpcium/commit/b5c15463fa01f58ce9557ebffaa0e96ce6dcda2d)
- Implement keyinfo store [View](https://github.com/cryptoniumX/mpcium/commit/d39168ddd7fc622eb1a58ea90617680249297515)
- Track ready nodes, reduce readiness period to 1 second [View](https://github.com/cryptoniumX/mpcium/commit/75cb2b286fffb89f1df39c680dee3d1cacfcffc9)
- Upgrade tss-lib to v2 [View](https://github.com/cryptoniumX/mpcium/commit/cd324358d7c297d2025ea2d0c02464b5552f513d)

## v0.1.1 (2024-02-05)

- Add retry package to improve NATs direct message resiliency [View](https://github.com/cryptoniumX/mpcium/commit/195f9a4c50732919994b67c13396f141fa4efcdf)
- Move listening to incoming message prior to genkey and signing to fix the nats: no responders available for request [View](https://github.com/cryptoniumX/mpcium/commit/421d02e947d12324c188d9bb2868cfb9ee02c3ca)
- Increase direct messsage timeout, add mutex lock for party updating [View](https://github.com/cryptoniumX/mpcium/commit/1d75eeea669212ff4b3168575cc07d4e8a0280ae)
- Pass done function to session to clean up resource after successful execution [View](https://github.com/cryptoniumX/mpcium/commit/e5430315a3ddc4a0b74b456ee9f4f68b185b5c5e)
- Implement queue manager to spawn message queue based on topic [View](https://github.com/cryptoniumX/mpcium/commit/7c107c9cba7db68358df77ea00b00d8d6b659d1d)
- Decode round8, round9 [View](https://github.com/cryptoniumX/mpcium/commit/ccd869e8d827ecfeae453b0f865899d35b520e0f)
- Refactor signing session [View](https://github.com/cryptoniumX/mpcium/commit/4447cc46da86c03a2353edc7f01b85fd79ead084)
- Clean up code [View](https://github.com/cryptoniumX/mpcium/commit/b40d8a42082ed00099054b9852a3415ca24426d0)
- Refactor keygen session [View](https://github.com/cryptoniumX/mpcium/commit/11bc34bb5831b0b3ad39fe14e77ef56a931d023d)
- Fix script to load peer ids to consul [View](https://github.com/cryptoniumX/mpcium/commit/70a1b53c350ce6414cca308aa588a53495c9411f)
