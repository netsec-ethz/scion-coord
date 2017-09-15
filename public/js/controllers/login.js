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
                        switch (response.data) {
                            case "Password invalid\n":
                                err = "Your username/password combination is incorrect. Please try again or reset the password with the link below.";
                                $scope.showReset = true;
                                break;
                            case "Email is not verified\n":
	                            $rootScope.resendAddress = user.email;
	                            $location.path('/resend');
                                break;
                            default:
                                err = "Failed to log you in: Make sure your email address and password are correct and your email address is verified.";
                        }
                        $scope.error = err;
                    });
            };

            // reset password
            $scope.resetPassword = function (email) {
                loginService.resetPassword(email).then(
                    function (data) {
                        $scope.error = "";
                        $scope.message = "Your password has been reset. You will receive an email with further instructions.";
                    },
                    function (response) {
                        console.log(response);
                        $scope.error = "An error occurred during the password reset procedure.";
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
