workflow "Run Tests and Release if Tagged" {
  on = "push"
  resolves = "Release"
}

action "Test" {
  uses = "seantis/roots/actions/test@master"
  runs = "run-tests"
}

action "Tagged" {
  needs = "Test"
  uses = "actions/bin/filter@master"
  args = "tag v*"
}

action "Release" {
  needs = "Tagged"
  uses = "seantis/roots/actions/release@master"
  runs = "run-release"
  secrets = ["GITHUB_TOKEN"]
}
