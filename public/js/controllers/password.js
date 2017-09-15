scionApp
    .controller('passwordCtrl', ['$scope', 'passwordService', '$routeParams', '$interval',
        '$location', function ($scope, passwordService, $routeParams, $interval, $location) {

            $scope.message = "";
            $scope.error = "";

            $scope.setPassword = function (user) {
              passwordService.setPassword(user).then(
                  function (data) {
                      console.log(data);
                      $scope.error = "";
                      $scope.message = "Your password was set successfully. You can now proceed " +
                          "to the login page.";
                      $scope.user = {uuid: $routeParams.uuid};
                      $scope.passwordForm.$setPristine(true)
                  },
                  function (response) {
                      console.log(response);
                      $scope.error = response.data;
                      $scope.message = "";
                  }
              )
            };

            $scope.user = {uuid: $routeParams.uuid};

            $scope.dismissSuccess = function () {
                $scope.message = "";
            };

            $scope.dismissError = function () {
                $scope.error = "";
            };

        }
    ]);
