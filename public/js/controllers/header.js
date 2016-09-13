angular.module('scionApp')
    .controller('headerCtrl', ['$scope', 'loginService', '$location', '$window', 
        function($scope, loginService, $location, $window) {

            $scope.isActive = function (viewLocation) {
                var active = (viewLocation === $location.path());
                return active;
            };

            $scope.logout = function() {            
                loginService.logout().then(
                    function(response) {                                            
                        $window.location.href = '/';
                    },
                    function(error) {                        
                        console.log(response);                        
                    });  
            };
 }]);