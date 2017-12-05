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
                        if (response.data.substring(0,3) === "900") {
                            $rootScope.resendAddress = user.email;
                            $location.path('/resend');
                        } else{
                            $scope.error = "Failed to log you in: Make sure your email address \
                                and password are correct and your email address is verified.";
                            $scope.message = "";
                        }
                    });
            };
 }]);
