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
                        $scope.error = "Login error. Please try again.";
                        console.log(response);
                        $scope.error = "Wrong email or password. Try again."
                    });  
            };

 }]);

