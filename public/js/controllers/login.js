scionApp
    .controller('loginCtrl', ['$rootScope', '$scope', 'loginService', '$location',
        function ($rootScope, $scope, loginService, $location) {

            // refresh the list of processes
            $scope.login = function (user) {
                if (!$scope.loginForm.$valid) {
                    $scope.error = "Please enter your email address and password."
                } else {
                    loginService.login(user).then(
                        function (data) {
                            $location.path('/user');
                        },
                        function (response) {
                            console.log(response);
                            $scope.error = "Failed to log you in: Make sure your email address and " +
                                "password are correct and your email address is verified.";
                            $scope.showReset = true;
                            $scope.showResend = true;
                            $scope.message = "";
                        });
                }
            };

            // reset password
            $scope.resetPassword = function (email) {
                if (!$scope.loginForm.$valid) {
                    $scope.error = "Please enter your email address."
                } else {
                    loginService.resetPassword(email).then(
                        function (data) {
                            $scope.error = "";
                            $scope.message = "Your password has been reset. You will receive an " +
                                "email with further instructions.";
                        },
                        function (response) {
                            console.log(response);
                            $scope.error = response.data;
                            $scope.message = "";
                        }
                    )
                }
            };

            // resend account verification email
            $scope.resendEmail = function (email){
                if (!$scope.loginForm.$valid) {
                    $scope.error = "Please enter your email address."
                } else {
                    loginService.resendEmail(email).then(
                        function (data) {
                            $scope.message = "The verification email has been resent to " + email;
                            $scope.error = "";
                        },
                        function (response) {
                            $scope.message = "";
                            $scope.error = response.data;
                        });
                }
            };

            $scope.dismissSuccess = function () {
                $scope.message = "";
            };

            $scope.dismissError = function () {
                $scope.error = "";
            };

        }]);
