scionApp
    .controller('accountCtrl', ['$scope', 'accountService', function ($scope, accountService) {

            $scope.message = "";
            $scope.error = "";

            $scope.changePassword = function (pwForm) {
                if (!$scope.passwordForm.$valid) {
                    $scope.error = "Please fill out the form correctly."
                } else {
                    accountService.changePassword(pwForm).then(
                        function (data) {
                            console.log(data);
                            $scope.error = "";
                            $scope.message = "Your password was changed successfully.";
                            $scope.pwForm = {};
                            $scope.passwordForm.$setPristine(true);
                        },
                        function (response) {
                            console.log(response);
                            $scope.error = response.data;
                            $scope.message = "";
                        }
                    )
                }
            };

            $scope.dismissSuccess = function () {
                $scope.message = "";
            };
            $scope.dismissError = function () {
                $scope.error = "";
            };

        }
    ]);
