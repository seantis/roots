workflow "Run Tests" {
  on = "push"
  resolves = "Test"
}

action "Test" {
  uses = "seantis/roots/actions/test@master"
  runs = "run-tests"
}
