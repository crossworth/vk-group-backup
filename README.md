## VK Group Backup

Downloads topics from a VK Group/Board/Community to JSON format.

### How to use
To run this project you must create a `settings.yml` file on the same directory of the executable.

**Exemple settings file**
```yaml
groupID: 0
mode: all
output: backup
continuousMode: true
accounts:
  - email: account1@email.com
    password: passwordForAccount1
  - email: account2@email.com
    password: passwordForAccount2
```

- groupID: The group ID from the VK Board
- mode: The download mode, `all` or `recents`
    - all: Download all topics
    - recents: Download the recents topics
- output: Folder where to place the files
- continuousMode: If the software must continue running after the fist pass.
- accounts: List of accounts to use to download the topics. You can provide how many accounts you got. The software will create
3 clients for each account. One client will be used to download the topic data and the rest for the comments.



After creating the file you can run the software and everything should be working fine.


*NOTE: If you are a group administrator you can use `LongPolling` from `VKAPI`, its better and easy than using this project.*
 
*NOTE: For the RabbitMQ version check the tag [v1.0.0](https://github.com/crossworth/vk-group-backup/tree/v1.0.0)*
