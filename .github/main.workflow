workflow "Run Tests" {
  on = "push"
  resolves = "Test"
}

action "Test" {
  uses = "seantis/roots/actions/test@master"
  runs = "run-tests"
}

workflow "Run Release" {
  on = "push"
  resolves = "Release"
}

action "Release" {
  needs = "Test"
  uses = "seantis/roots/actions/release@master"
  runs = "run-release"
  secrets = ["GITHUB_TOKEN"]
}
