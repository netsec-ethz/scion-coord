scionApp
    .controller('loginCtrl', ['$rootScope', '$scope', 'loginService', '$location',
        function ($rootScope, $scope, loginService, $location) {

            // refresh the list of processes
            $scope.login = function (user) {

                loginService.login(user).then(
                    function (data) {
                        $location.path('/user');
                    },
                    function (response) {
                        console.log(response);

                        let err;
                        switch (response.data.substring(0,3)) {
                            case "901":
                                err = "Your username/password combination is incorrect. Please " +
                                    "try again or reset the password with the link below.";
                                $scope.showReset = true;
                                break;
                            case "900":
	                            $rootScope.resendAddress = user.email;
	                            $location.path('/resend');
                                break;
                            case "902":
                                err = "You have not set a valid password. Please check your " +
                                    "email and follow the link to set a new password.";
                                break;
                            case "903":
                                err = "Your account is not yet activated. You " +
                                    "receive an email as soon as your account is approved.";
                                break;
                            default:
                                err = "Failed to log you in: Make sure your email address and " +
                                    "password are correct and your email address is verified.";
                        }
                        $scope.error = err;
                        $scope.message = "";
                    });
            };

            // reset password
            $scope.resetPassword = function (email) {
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
            };

            $scope.dismissSuccess = function () {
                $scope.message = "";
            };

            $scope.dismissError = function () {
                $scope.error = "";
            };
        }]);
