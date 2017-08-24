angular.module('scionApp')
    .controller('loginCtrl', ['$scope', 'loginService', '$location',
        function($scope, loginService, $location) {

            // refresh the list of processes
            $scope.login = function (user) {

                loginService.login(user).then(
                    function(data) {
                        $location.path('/admin');
                    },
                    function(response) {
                        console.log(response);
                        $scope.error = "Failed to log you in: Make sure your email address and password are correct and your email address is verified."
                        $scope.message = "";
                    });
            };

            $scope.resendEmail = function(email){
                if (!email){
                    $scope.error = "Please fill out the email field."
                } else {

                    loginService.resendEmail(email).then(function (response){
                        $scope.message = "Verification email resent.";
                        $scope.error = "";
                    },
                    function(response){
                        $scope.message = "";
                        $scope.error = response.data;
                    });

                }
            };

 }]);
