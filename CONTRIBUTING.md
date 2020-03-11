# Contributing to CF-Operator

First, thanks for taking the time to contribute to the project!

The following is a set of guidelines for contributing to CF-Operator. These are mostly guidelines,
not rules. Use your best judgment, and feel free to propose changes to this document in a pull
request.

## How to contribute

Before kicking the ball, please **do NOT report security vulnerabilities in public issues**!
Instead, report with the CloudFoundry Foundation team first <security@cloudfoundry.org> and give us
some time to fix it in a fashion time matter before disclosing it to the public. For more
information check the CloudFoundry Security [page](https://www.cloudfoundry.org/security/).

Don't forget to check more in-depth information that supports many of the tasks, like the development or release process in [here](doc/README.md).

### Conversation

When contributing to this repository, please first discuss the changes through an existing Github issue with the main contributors. We do believe is the right place to have an open conversation and a better
final decision.

### Pull Request

Pull Requests are the most well-known way to contribute to any project and they are more than welcome in the CF-Operator project but, like any other community project, the commit messages must be clear and concise to help the reviewer in understanding the challenge and the solution that the PR is trying to address.

## Issues tracker

The CF-Operator workload is tracked [here](https://www.pivotaltracker.com/n/projects/2192232)
and all the community iteration will happen on Github [repo][3] through Github issues, either bugs or features.

We're committed to having short and accurate templates for different types of issues, to gather the required information to start a conversation. Once again, feel free to contribute to improving them by opening a pull request.

### How to report a bug

Start by searching [bugs][1] for some similar issues and/or extra information. If your search
doesn't bring you any help then open an issue by selecting the issue type "Bug" and fill the
template accurately as possible.

### How to suggest a feature/enhancement

If you find yourself wishing for a feature/enhancement that does not exist yet in CF-Operator start
by running a quick [search][2] - you may not be alone! If you ain't any luck, then go ahead and open a
new issue by selecting the "Feature" issue type and answer some needed questions.

### How are Github issues handled

When you create a github issue, cf-bot creates a story in the `IceBox` board in the tracker automatically and comments on the issue with the link to the story. The team conducts its planning session once every week during which your issue will be discussed.

* If the story is moved to the `Current Iteration/Backlog` board, then expect that the story would be worked on in the coming sprints.
* If the story is left out in the `IceBox` board itself, then expect that the story is not in priority and the github issue will be closed. This doesn't mean we reject to implement the feature or fix. This step is to prevent piling up of github issues's. The story is still in the tracker and will be worked on when it is moved from `IceBox` to `Current Iteration/Backlog` during planning sessions.
* Both github issue and the story will be closed if there is no response for more than 30 days from the person who filed it, when contacted.

### How are tracker stories handled

* By default, a story is in `Unstarted` state. 
* When the developer clicks on the `Start` button, the story moves to `Started` state. 
* When the developer finishes the story, submits a PR and clicks on `Finish` button, the story moves to `Finished` state.
* After approving all PRs that belong to a story, the reviewer and author then try to merge and rebase those PRs, changing references as needed, and ensure all tests are still green. Finally one of them clicks the `Deliver` button which is when the story moves to the `Delivered` state. 
* The team lead after checking the feature/bugfix, will accept the story. That is the end of the life of a story. A detailed flow diagram can be found [here](https://www.pivotaltracker.com/help/articles/story_states/).

## Code review process

The core team looks to Pull Requests regularly and in a best effort.

## Community

You can chat with the core team on Slack channel #quarks-dev.

## Code of conduct

Please refer to [Code of Conduct](https://www.cloudfoundry.org/code-of-conduct/)

## Links

- [Bugs][1]
- [Features][2]
- [Readme][3]

[1]: https://github.com/cloudfoundry-incubator/cf-operator/issues?q=is%3Aopen+is%3Aissue+label%3Abug

[2]: https://github.com/cloudfoundry-incubator/cf-operator/issues?q=is%3Aopen+is%3Aissue+label%3Aenhancement

[3]: https://github.com/cloudfoundry-incubator/cf-operator#cf-operator
