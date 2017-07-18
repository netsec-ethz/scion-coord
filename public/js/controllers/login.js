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
                    });  
            };

 }]);

